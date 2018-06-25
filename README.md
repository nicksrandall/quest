# Quest
[![GoDoc](https://godoc.org/github.com/nicksrandall/quest?status.svg)](https://godoc.org/github.com/nicksrandall/quest)
A request (http) library for Go with a convenient, chain-able API.

## Errors
If any method in the request's life-cycle errors, the every subsequent method will short circut and not be called. The `Done` method will return the first error that occured so that it can be handled.

## Example
```go
func GetUser(ctx context.Context, userId string) (user.User, error) {
	var user user.User
	err := quest.Post("/api/to/user/endpoint").
		WitContext(ctx). // used for open tracing
		QueryParam("includeDetails", "true").
		JSONBody(userId).
		Send().
		ExpectSuccess().
		GetJSON(&user).
		Done()

  return  user, err
}
```

## Open Tracing
- [x] this library will work with [Open Tracing](https://github.com/opentracing/opentracing-go)
