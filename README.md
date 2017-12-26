# Guldan Go Client

# API

## `NewGuldanClient() *GuldanClient`
> 创建`guldan`客户端实例，如果没有特殊需求，不建议使用

## `GetInstance() *GuldanClient`
> 获取`guldan`客户端单例，没有特殊需求请使用这个方法获取实例

## `func (c *GuldanClient) SetAddress(address string)`
> 设置`guldan`地址，默认`guldan`地址为国内的`guldan`地址，如果需要访问国外或者代理服务需要设置设置地址

## `func (c *GuldanClient) SetRole(role string)`
> 设置使用该客户端的角色，默认为`client`，该字段将可以在webui中展示

## `func (c *GuldanClient) SetItemExpireInterval(interval int32)`
> 设置每个多久和服务端同步一次配置，该配置项当且仅当使用了`Watch/WatchPublic`时有效，默认时间为5秒

## `func (c *GuldanClient) SetMissCache(interval int32)`
> 设置不存在数据的cache时间，可以减少对远端服务器不必要的请求，默认为关闭该特性

## `func (c *GuldanClient) SetPrinter(printer PrintCallback)`
> 可以通过设置`printer`来调试访问`guldan`异常，一般不需要设置

## `func (c *GuldanClient) Get(gid, token string, cached bool, gray bool) (string, error)`
> 主动获取配置

### 参数

- `gid` 你的配置唯一ID，比如"public.hello.world"
- `token` 你的用户唯一Token，如果是公开的，可以传""，也可以使用`GetPublic`
- `cached` 表示你是否会从本地cache中去该配置，如果设置成`false`，每次调用这个接口都将发生一次网络请求，如果设置`true`将会优先取本地缓存，如果缓存不存在走网络
- `gray` 表示是否获取灰度配置，如果不是灰度发布，请填`false`

### 返回值
- `string` 你的配置内容，如果异常或不存在或无权限，该字段为`nil`
- `error`  异常信息，这个`string`字段不是`nil`，这个字段必然`nil`，反之亦然

## `func (c *GuldanClient) GetPublic(gid string, cached bool, gray bool) (string, error)`
> `Get`接口的拿公开配置的封装，参数以及返回同`Get`

## `func (c *GuldanClient) RawGet(gid, token string, cached bool, gray bool) (*Item, error)`
> `Get`接口的底层实现，参数同`Get`，返回值第一个返回值为对应的配置项结构体，没有特殊情况不推荐使用

## `func (c *GuldanClient) Watch(gid, token string, gray bool, notify NotifyCallback, checker CheckCallback) error`
> 设置订阅配置变更，只有设置了`Watch`才会后台实时更新配置，需要注意的一点是在使用`Get/GetPublic`时设置了`cached=true`，但没有使用`Watch/WatchPublic`将无法实时更新本地的配置

### 参数

- `gid` 你的配置唯一ID，比如"public.hello.world"
- `token` 你的用户唯一Token，如果是公开的，可以传""，也可以使用`GetPublic`
- `gray` 表示是否获取灰度配置，如果不是灰度发布，请填`false`
- `notify` 配置变更回调，如果设置为`nil`，将不会通知你，但是依然会更新本地的配置，也就是你下次`Get/GetPublic`是可以获取到最新配置的
- `checker` 配置内容检查器，主要用于防止有人不小心改错了配置同时又发布的情况，可以通过checker在本地验证是否符合业务的要求，如果设置为`nil`，将不会对检查返回的配置内容，如果不为nil，如果在该回调中返回`true`将会将配置缓存在本地，如果返回`false`，将不会在本地缓存该变更

### 返回值
- `error` 成功该返回值为`nil`，否则为相关错误

## `func (c *GuldanClient) WatchPublic(gid string, gray bool, notify NotifyCallback, checker CheckCallback) error`
> `Watch`接口的订阅空开配置的封装，参数以及返回同`Watch`

## `func (c *GuldanClient) CachedCount() int`
> 返回本地返回的缓存配置项数

# 特殊Error

> 特殊错误主要以下3中都是由于用户导致的 其他的error都为系统导致的 比如断网/服务端宕机之类的

## `ErrGuldanNotFound`
表示要获取的配置不存在

## `ErrGuldanForbidden`
表示你没有权限访问该配置或者你的token写错了

## `ErrGuldanBadConfigFormat`
表示配置符合你的要求 只有在`Watch/WatchPublic`设置了参数`checker`的时候在`notify`中出现，且只有的checker返回`error`时会出现
