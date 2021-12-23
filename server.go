package ginrpc

import (
	"context"
	"github.com/alphaqiu/ginrpc/payload"
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
	Start(runMode string, sig ...os.Signal) <-chan os.Signal
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

func (g *ginServer) Start(runMode string, sig ...os.Signal) <-chan os.Signal {
	gin.SetMode(runMode)
	ch := make(chan os.Signal)

	g.router.Use(g.preInterceptors...)
	g.makeRoutes()
	g.router.Use(g.postInterceptors...)

	go g.listenAndServe()
	signal.Notify(ch, sig...)
	return ch
}

func (g *ginServer) listenAndServe() {
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

	log.Info("http 正常退出")
}

func (g *ginServer) Stop(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	go func() {
		if err := g.httpServer.Shutdown(ctx); err != nil {
			log.Errorf("关闭http服务遇到了错误: %v", err)
		}
	}()

	select {
	case <-time.After(12 * time.Second):
		log.Warn("停止http服务超时, 服务将退出")
	case <-cctx.Done():
		if cctx.Err() != nil {
			return errors.Wrap(cctx.Err(), "停止http遇到了错误")
		}
		log.Info("http服务已停止")
	}

	return nil
}

func (g *ginServer) Bind(service interface{}) error {
	// 结构体名称作为资源名称，方法默认都是POST，如果前缀为Get，则是Get，前缀为Options 则是Options
	// action=去掉前缀的方法名
	// 入參支持绑定JSON和Query，如果入參结构体后缀为Query，则以Query方式解析
	// 出參最多支持3个参数，最后一个参数必须是error，或者实现了error接口的结构体
	svcRef := reflect.ValueOf(service)
	if reflect.Ptr != svcRef.Kind() || svcRef.Elem().Kind() != reflect.Struct {
		return errors.Wrapf(invalidInstanceErr, "%+v", svcRef)
	}
	log.Debugf("开始绑定服务: %+v", svcRef.Type())
	start := time.Now()

	actions := make(map[string]*actionInOutParams)
	for f := 0; f < svcRef.NumMethod(); f++ {
		method := svcRef.Type().Method(f)
		if !method.IsExported() {
			log.Debugf("[1]非Service方法. 无效的方法. 方法签名: %s", method.Type)
			continue
		}

		reqMethod, resourceName, actionName := parseMethodName(svcRef.Elem().Type(), method.Name)
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
		numOutParams, ok := g.checkOutParams(method.Type)
		if !ok {
			continue
		}

		if actionInOutParam := g.initInParams(method); actionInOutParam != nil {
			actionInOutParam.ReqMethod = reqMethod
			actionInOutParam.ResourceName = resourceName
			actionInOutParam.OutParamNum = numOutParams
			actionInOutParam.Self = svcRef
			actionInOutParam.Fn = method.Func
			actionInOutParam.ActionName = actionName
			actions[method.Name] = actionInOutParam
		}
	}

	for _, inOutParams := range actions {
		handler := g.assignHandler(inOutParams)
		relativePath := g.relativePath(inOutParams.ResourceName, inOutParams.ActionName)
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
		for _, item := range g.services {
			apis = append(apis, item.RelativePath)
		}

		c.JSON(http.StatusOK, gin.H{"apis": apis})
	})
}

func (g *ginServer) checkOutParams(methodType reflect.Type) (numParams int, ok bool) {
	outCount := methodType.NumOut()
	if outCount < 1 || outCount > 2 {
		// 不符合service的方法，结构体可以定义非服务方法，这类方法过滤，不返回错误
		log.Debugf("[0-1]非Service方法，无效的返回值. return params: %d, %v", outCount, methodType)
		return
	}

	item := methodType.Out(outCount - 1)
	kind := item.Kind()
	if kind != reflect.Interface && !item.Implements(respInterface) {
		log.Debugf("[0-2]非Service方法, 无效的返回值. 最后一个参数不是payload.Response: %v", kind)
		return
	}

	if outCount == 2 {
		item = methodType.Out(0)
		if !g.checkResultParam(item) {
			// 当返回值=2时，第一个参数必须时结构体
			log.Debugf("[0-3]非Service方法, 无效的返回值. 返回的第一个参数不是结构体: %v", item.Kind())
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
	if kind != reflect.Struct {
		return false
	}

	return true
}

func (g *ginServer) initInParams(method reflect.Method) *actionInOutParams {
	inCount := method.Type.NumIn()
	if inCount > 4 { // 包含方法所属自身引用
		log.Debugf("[2]非Service方法. 无效的入參. 方法签名: %s", method.Type)
		return nil
	}

	inParam := new(actionInOutParams)
	for p := 1; p < method.Type.NumIn(); p++ {
		param := method.Type.In(p)
		if param.Kind() == reflect.Ptr {
			param = param.Elem()
		}

		isHeader := g.isHttpHeaderSignature(param)
		if param.Kind() != reflect.Struct && !isHeader {
			log.Debugf("[3]非Service方法. 无效的入參. 方法签名: %s", method.Type)
			return nil
		}

		if isHeader {
			inParam.HasHeader = true
			inParam.HeaderIndex = p
		} else if strings.HasSuffix(param.Name(), "Query") {
			inParam.Query = reflect.New(param)
			inParam.QueryKind = method.Type.In(p).Kind()
			inParam.QueryIndex = p
			inParam.HasQuery = true
		} else {
			inParam.Body = reflect.New(param)
			inParam.BodyKind = method.Type.In(p).Kind()
			inParam.BodyIndex = p
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
		inParams[0] = inOutParam.Self
		if inOutParam.HasHeader {
			inParams[inOutParam.HeaderIndex] = reflect.ValueOf(ctx.Request.Header)
		}

		if inOutParam.HasQuery {
			err = ctx.BindQuery(inOutParam.Query.Interface())
			if err != nil {
				ctx.Abort()
				ctx.JSON(http.StatusBadRequest, gin.H{})
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
				ctx.JSON(http.StatusBadRequest, gin.H{"errMsg": err.Error()})
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

		re := iResp.(payload.Response)
		if re.GetErr() == nil {
			if g.cnf.SuccessResponseFunc != nil {
				g.cnf.SuccessResponseFunc(ctx, iResp.(payload.Response), result)
				return
			}
			g.defaultSuccessResponse(ctx, iResp.(payload.Response), result)
			return
		}

		if g.cnf.ErrResponseFunc != nil {
			g.cnf.ErrResponseFunc(ctx, iResp.(payload.Response), result)
			return
		}
		g.defaultErrResponse(ctx, iResp.(payload.Response), result)
	}
}

func (g *ginServer) defaultSuccessResponse(ctx *gin.Context, resp payload.Response, data interface{}) {
	httpCode := resp.GetCode()
	header := resp.GetHeader()
	setHeader(ctx, header)

	if data == nil {
		ctx.AbortWithStatus(httpCode)
		return
	}

	ctx.JSON(httpCode, gin.H{"result": data})
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

func (g *ginServer) defaultErrResponse(ctx *gin.Context, resp payload.Response, data interface{}) {
	httpCode := resp.GetCode()
	header := resp.GetHeader()
	err := resp.GetErr()
	setHeader(ctx, header)

	ret := gin.H{"errMsg": err.Error()}
	if data != nil {
		ret["result"] = data
	}

	ctx.JSON(httpCode, ret)
}

func (g *ginServer) relativePath(resourceName, actionName string) string {
	if len(g.cnf.UrlPrefix) > 0 {
		return strings.Join([]string{g.cnf.UrlPrefix, resourceName, actionName}, "/")
	}

	return strings.Join([]string{resourceName, actionName}, "/")
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
	Self         reflect.Value
	Fn           reflect.Value
}
