结构体名称作为资源名称，方法默认都是POST，如果前缀为Get，则是Get，前缀为Options 则是Options
action=去掉前缀的方法名
入參支持绑定JSON和Query，如果入參结构体后缀为Query，则以Query方式解析
出參最多支持3个参数，最后一个参数必须是error，或者实现了error接口的结构体

contentParam: body 内部的数据绑定，可以是application/json,可以是multipart/form-data,可以是application/x-www-form-urlencoded
 也可以是ProtoBuf和msgPack 消息格式。由Header: Content-Type 决定
 queryParam: url参数绑定
 服务的方法签名

```go

 func() payload.Response
 func() (result, payload.Response)
 func(contentParam) payload.Response
 func(contentParam) (result, payload.Response)
 func(queryParam) payload.Response
 func(queryParam) (result, payload.Response)
 func(queryParam, contentParam) payload.Response
 func(queryParam, contentParam) (result, payload.Response)
 func(header) payload.Response
 func(queryParam, header) payload.Response
 func(queryParam, contentParam, header) payload.Response
 func(contentParam, header) payload.Response
```