package ginrpc

import (
	"context"
	"github.com/gin-gonic/gin"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"time"
)

var log = logging.Logger("ginrpc")

func New(cnf *Config) APIServer {
	if cnf == nil {
		cnf = defaultConfig()
	}

	r := gin.New()
	httpServer := &http.Server{
		Addr:         cnf.Addr,
		Handler:      r,
		ReadTimeout:  cnf.ReadTimeout,
		WriteTimeout: cnf.WriteTimout,
		IdleTimeout:  cnf.IdleTimeout,
	}

	if cnf.KeepAlive {
		httpServer.SetKeepAlivesEnabled(true)
	}

	return &ginServer{cnf: cnf, router: r, httpServer: httpServer}
}

type APIServer interface {
	Start(sig ...os.Signal) (context.Context, <-chan os.Signal)
	Stop(ctx context.Context) error
	BindPreInterceptor(handlerFuncs ...gin.HandlerFunc)
	Bind(interface{}) error
	BindPostInterceptor(handlerFuncs ...gin.HandlerFunc)
}

type ginServer struct {
	cnf              *Config
	router           *gin.Engine
	httpServer       *http.Server
	preInterceptors  []gin.HandlerFunc
	postInterceptors []gin.HandlerFunc
	services         []serviceMap
}

func (g *ginServer) BindPreInterceptor(handlerFuncs ...gin.HandlerFunc) {
	g.preInterceptors = append(g.preInterceptors, handlerFuncs...)
}

func (g *ginServer) BindPostInterceptor(handlerFuncs ...gin.HandlerFunc) {
	g.postInterceptors = append(g.postInterceptors, handlerFuncs...)
}

func (g *ginServer) Start(sig ...os.Signal) (context.Context, <-chan os.Signal) {
	gin.SetMode(g.cnf.RunMode)
	ch := make(chan os.Signal)

	g.router.Use(g.preInterceptors...)
	g.makeRoutes()
	g.router.Use(g.postInterceptors...)

	ctx, cancel := context.WithCancel(context.Background())
	go g.listenAndServe(cancel)
	signal.Notify(ch, sig...)
	return ctx, ch
}

func (g *ginServer) listenAndServe(cancel context.CancelFunc) {
	defer cancel()
	log.Info("http 服务启动中...")

	if g.cnf.Tls != nil && g.cnf.Tls.Enabled {
		if len(g.cnf.Tls.Redirect) > 0 {
			go func() {
				log.Debug("TLS Redirect ON.")
				err := http.ListenAndServe(g.cnf.Tls.Redirect, tlsRedirect(g.cnf.Addr))
				if err != nil {
					log.Errorf("HTTP重定向端口故障退出: %v", errors.WithStack(err))
				}
			}()
		}

		g.httpServer.TLSConfig = makeTls(g.cnf.Tls)
		// cert & key already made in tlsconfig
		if err := g.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Errorf("TLS服务异常退出: %+v", errors.WithStack(err))
			return
		}
	} else {
		// service connections
		if err := g.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("HTTP服务异常退出: %+v", errors.WithStack(err))
			return
		}
	}
}

func (g *ginServer) Stop(ctx context.Context) error {
	timeout := g.cnf.ShutdownTimeout
	delay := timeout + 2*time.Second
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	go func() {
		if err := g.httpServer.Shutdown(ctx); err != nil {
			log.Errorf("关闭http服务遇到了错误: %v", err)
		}
	}()

	select {
	case <-time.After(delay):
		log.Warn("停止http服务超时, 服务将退出")
	case <-cctx.Done():
		if cctx.Err() != nil {
			return errors.Wrap(cctx.Err(), "停止http遇到了错误")
		}
		log.Info("http服务已停止, 正常退出")
	case <-ctx.Done():
		log.Info("http服务已停止, 正常退出")
		return nil
	}

	return nil
}

func (g *ginServer) Bind(service interface{}) error {
	// 结构体名称作为资源名称，方法默认都是POST，如果前缀为Get，则是Get，前缀为Options 则是Options
	// action=去掉前缀的方法名
	// 入參支持绑定JSON和Query，如果入參结构体后缀为Query，则以Query方式解析
	// 出參最多支持3个参数，最后一个参数必须是error，或者实现了error接口的结构体
	version := "v0"
	if vo, ok := service.(ResourceVersion); ok {
		version = strings.ReplaceAll(strings.Trim(vo.Version(), " "), " ", "_")
	}

	svcRef := reflect.ValueOf(service)
	if reflect.Ptr != svcRef.Kind() || svcRef.Elem().Kind() != reflect.Struct {
		return errors.Wrapf(invalidInstanceErr, "%+v", svcRef)
	}
	log.Debugf("开始绑定服务: %+v", svcRef.Type())
	start := time.Now()

	actions := make(map[string]*actionInOutParams)
	for f := 0; f < svcRef.NumMethod(); f++ {
		methodInst := svcRef.Method(f)
		methodDef := svcRef.Type().Method(f)
		//if !method.IsExported() {
		//	log.Debugf("[1]非Service方法. 无效的方法. 方法签名: %s", method.Type)
		//	continue
		//}

		reqMethod, resourceName, actionName := parseMethodName(svcRef.Elem().Type(), methodDef.Name)
		log.Debugf("HTTP Method: %s, %s/%s/%s", reqMethod, g.cnf.UrlPrefix, resourceName, actionName)

		// contentParam: body 内部的数据绑定，可以是application/json,可以是multipart/form-data,可以是application/x-www-form-urlencoded
		// 也可以是ProtoBuf和msgPack 消息格式。由Header: Content-Type 决定
		// queryParam: url参数绑定
		// 服务的方法签名
		// func() ginrpc.Response
		// func() (result, ginrpc.Response)
		// func(contentParam) ginrpc.Response
		// func(contentParam) (result, ginrpc.Response)
		// func(queryParam) ginrpc.Response
		// func(queryParam) (result, ginrpc.Response)
		// func(queryParam, contentParam) ginrpc.Response
		// func(queryParam, contentParam) (result, ginrpc.Response)
		// func(header) ginrpc.Response
		// func(queryParam, header) ginrpc.Response
		// func(queryParam, contentParam, header) ginrpc.Response
		// func(contentParam, header) ginrpc.Response
		numOutParams, ok := g.checkOutParams(methodDef.Type, methodInst)
		if !ok {
			continue
		}

		if actionInOutParam := g.initInParams(methodDef, methodInst); actionInOutParam != nil {
			actionInOutParam.ReqMethod = reqMethod
			actionInOutParam.ResourceName = resourceName
			actionInOutParam.OutParamNum = numOutParams
			actionInOutParam.Fn = methodInst
			actionInOutParam.ActionName = actionName
			actions[methodDef.Name] = actionInOutParam
		}
	}

	for _, inOutParams := range actions {
		handler := g.assignHandler(inOutParams)
		relativePath := g.relativePath(version, inOutParams.ResourceName, inOutParams.ActionName)
		g.services = append(g.services, serviceMap{
			Method:       inOutParams.ReqMethod,
			RelativePath: relativePath,
			Func:         handler,
		})
	}

	log.Debugf("服务绑定完成: %s 核计绑定了%d个服务接口, time elapsed: %s",
		svcRef.Type(),
		len(actions),
		time.Now().Sub(start))
	return nil
}

func (g *ginServer) makeRoutes() {
	for _, item := range g.services {
		log.Debugf("Method: %s, Path: %s", item.Method, item.RelativePath)
		g.router.Handle(item.Method, item.RelativePath, item.Func)
	}

	api := "/exports"
	if len(g.cnf.UrlPrefix) > 0 {
		api = g.cnf.UrlPrefix + api
	}

	g.router.Handle(http.MethodGet, api, func(c *gin.Context) {
		apis := make([]string, len(g.services))
		for idx, item := range g.services {
			apis[idx] = item.RelativePath
		}

		c.JSON(http.StatusOK, gin.H{"apis": apis})
	})
}

func (g *ginServer) checkOutParams(methodType reflect.Type, methodInst reflect.Value) (numParams int, ok bool) {
	outCount := methodType.NumOut()
	if outCount < 1 || outCount > 2 {
		// 不符合service的方法，结构体可以定义非服务方法，这类方法过滤，不返回错误
		log.Debugf("[0-1]非Service方法，无效的返回值. return params: %d, %v", outCount, methodType)
		return
	}

	item := methodType.Out(outCount - 1)
	kind := item.Kind()
	if kind != reflect.Interface && !item.Implements(errInterface) {
		log.Debugf("[0-2]非Service方法, 无效的返回值. 最后一个参数不是实现了error的接口: Kind: %v, 方法签名: %s", kind, methodInst.Type())
		return
	}

	if outCount == 2 {
		item = methodType.Out(0)
		if !g.checkResultParam(item) {
			// 当返回值=2时，第一个参数必须时结构体或者Slice
			log.Debugf("[0-3]非Service方法, 无效的返回值. 返回的第一个参数不是结构体或者Slice: %v", item.Kind())
			return
		}
		return 2, true
	}
	return 1, true
}

func (g *ginServer) checkResultParam(item reflect.Type) bool {
	kind := item.Kind()
	if kind == reflect.Ptr {
		item = item.Elem()
	}

	kind = item.Kind()
	if kind != reflect.Struct && kind != reflect.Slice {
		return false
	}

	return true
}

func (g *ginServer) initInParams(method reflect.Method, methodInst reflect.Value) *actionInOutParams {
	inCount := method.Type.NumIn()
	if inCount < 2 || inCount > 5 { // 包含方法所属自身引用
		log.Debugf("[1]非Service方法. 无效的入參. 方法签名: %s", methodInst.Type())
		return nil
	}

	param := method.Type.In(1)
	ctxRef := reflect.TypeOf((*context.Context)(nil)).Elem()
	if param.Kind() != reflect.Interface && !param.Implements(ctxRef) {
		log.Debugf("[2]非Service方法. 无效的入參.第一个参数应为Context: Kind: %s, 方法签名: %s", param.Kind(), methodInst.Type())
		return nil
	}

	inParam := new(actionInOutParams)
	for p := 2; p < method.Type.NumIn(); p++ {
		param = method.Type.In(p)
		if param.Kind() == reflect.Ptr {
			param = param.Elem()
		}

		isHeader := g.isHttpHeaderSignature(param)
		if param.Kind() != reflect.Struct && !isHeader {
			log.Debugf("[3]非Service方法. 无效的入參. 方法签名: %s", methodInst.Type())
			return nil
		}

		if isHeader {
			inParam.HasHeader = true
			inParam.HeaderIndex = p - 1
		} else if strings.HasSuffix(param.Name(), "Query") {
			inParam.Query = reflect.New(param)
			inParam.QueryKind = method.Type.In(p).Kind()
			inParam.QueryIndex = p - 1
			inParam.HasQuery = true
		} else {
			inParam.Body = reflect.New(param)
			inParam.BodyKind = method.Type.In(p).Kind()
			inParam.BodyIndex = p - 1
			inParam.HasBody = true
		}
	}

	return inParam
}

func (g *ginServer) isHttpHeaderSignature(ht reflect.Type) bool {
	if ht.Kind() == reflect.Map && ht.Name() == "Header" && ht.PkgPath() == "net/http" {
		return true
	}

	return false
}

func (g *ginServer) assignHandler(inOutParam *actionInOutParams) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var e interface{}
				if reflect.TypeOf(err).Implements(errInterface) {
					e = errors.WithStack(err.(error)).Error()
				} else {
					e = err
				}
				log.Errorf("调用Service服务遇到了问题:(%s;%s/%s) %v",
					inOutParam.ReqMethod, inOutParam.ResourceName, inOutParam.ActionName,
					e)
			}
		}()
		// gin框架会自动判断绑定的类型，这里只要区分是否含有Query和body内的绑定。
		log.Debugf("开始调用: Method: %s; %s/%s", inOutParam.ReqMethod, inOutParam.ResourceName, inOutParam.ActionName)
		var (
			err      error
			inParams []reflect.Value
		)

		paramsLen := 1
		if inOutParam.HasHeader {
			paramsLen += 1
		}

		if inOutParam.HasQuery {
			paramsLen += 1
		}

		if inOutParam.HasBody {
			paramsLen += 1
		}

		inParams = make([]reflect.Value, paramsLen)

		parentCtx := ctx.Request.Context()
		if parentCtx.Done() == nil {
			parentCtx = ctx
		}

		inParams[0] = reflect.ValueOf(parentCtx)
		if inOutParam.HasHeader {
			inParams[inOutParam.HeaderIndex] = reflect.ValueOf(ctx.Request.Header)
		}

		if inOutParam.HasQuery {
			err = ctx.BindQuery(inOutParam.Query.Interface())
			if err != nil {
				ctx.Abort()
				ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "failed to bind params in query", "error": err.Error()})
				return
			}

			q := inOutParam.Query
			if inOutParam.QueryKind != reflect.Ptr {
				q = inOutParam.Query.Elem()
			}
			inParams[inOutParam.QueryIndex] = q
		}

		if inOutParam.HasBody {
			err = ctx.Bind(inOutParam.Body.Interface())
			if err != nil {
				ctx.Abort()
				ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "failed to bind params in body", "error": err.Error()})
				return
			}

			b := inOutParam.Body
			if inOutParam.BodyKind != reflect.Ptr {
				b = inOutParam.Body.Elem()
			}
			inParams[inOutParam.BodyIndex] = b
		}

		log.Debugf("Call Params: %d, %+v", len(inParams), inParams)
		ret := inOutParam.Fn.Call(inParams)
		log.Debugf("End Fn.Call, in: %v, out: %v", inParams, ret)
		defer log.Debugf("结束调用: Method: %s; %s/%s", inOutParam.ReqMethod, inOutParam.ResourceName, inOutParam.ActionName)

		var (
			result interface{}
			iResp  interface{}
		)

		if inOutParam.OutParamNum == 1 {
			result = nil
			iResp = ret[0].Interface()
		} else { // 2
			result = ret[0].Interface()
			iResp = ret[1].Interface()
		}

		if iResp == nil {
			g.defaultResponse(ctx, result, nil)
			return
		}

		if re, ok := iResp.(Err); ok {
			g.defaultResponse(ctx, result, re)
			return
		}

		// internal error 未定义的错误
		if re, ok := iResp.(error); ok {
			g.defaultResponse(ctx, result, &internalError{error: re})
			return
		}

		panic("unreachable code")

	}
}

func (g *ginServer) defaultResponse(ctx *gin.Context, data interface{}, resp Err) {
	ret := gin.H{}
	if resp == nil && data == nil {
		ctx.AbortWithStatus(http.StatusOK)
		return
	}

	if data != nil {
		ret["result"] = data
	}

	var (
		code    int
		message string
		errMsg  string
	)
	if resp != nil {
		code = resp.Code()
		message = resp.Message()
		errMsg = resp.Error()
	}

	//setHeader(ctx, header)

	if code > 0 {
		ret["code"] = code
	}

	if message != "" {
		ret["message"] = message
	}

	if errMsg != "" {
		ret["error"] = errMsg
	}

	if len(ret) == 0 {
		ctx.AbortWithStatus(http.StatusOK)
		return
	}

	ctx.JSON(http.StatusOK, ret)
}

func setHeader(ctx *gin.Context, header http.Header) {
	if header != nil {
		for k, vs := range header {
			for _, v := range vs {
				ctx.Header(k, v)
			}
		}
	}
}

func (g *ginServer) relativePath(version, resourceName, actionName string) string {
	if len(g.cnf.UrlPrefix) > 0 {
		return strings.Join([]string{g.cnf.UrlPrefix, version, resourceName, actionName}, "/")
	}

	return strings.Join([]string{version, resourceName, actionName}, "/")
}

type serviceMap struct {
	Method       string
	RelativePath string
	Func         gin.HandlerFunc
}

type actionInOutParams struct {
	HasQuery     bool
	HasBody      bool
	HasHeader    bool
	HeaderIndex  int
	Query        reflect.Value
	QueryKind    reflect.Kind
	QueryIndex   int
	Body         reflect.Value
	BodyKind     reflect.Kind
	BodyIndex    int
	OutParamNum  int
	ReqMethod    string
	ResourceName string
	ActionName   string
	Fn           reflect.Value
}
