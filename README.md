# Quest
A request (http) library for Go with a convenient, chain-able API.

## Example
```go
func GetUser(ctx context.Context, userId string) (user.User, error) {
	var user user.User
	err := quest.Post("/api/to/user/endpoint").
		SetContext(ctx).
		QueryParam("includeDetails", "true").
		JSONBody(userId).
		Send().
		ExpectSuccess().
		GetJSON(&user).
		Done()
  }

  return  user, err
}
```

## Extending the API
Embedding FTW!

```go
type SpecialClient struct {
  quest.ClientInterface
}

func (c *SpecialClient) New(method string, path string) quest.RequestInterface {
  req := c.ClientInterface.New(method, path)
  return &SpecialRequest{req}
}

type SpecialRequest struct {
  quest.RequestInterface
}

// Extend the api to add and `Auth` method
func (r *SpecialRequest) Auth(key string) quest.RequestInterface {
  // some fake auth logic
  r.Header("X-Some-Authentication", key)
  return r
}

type SpecialResponse struct {
  quest.ResponseInterface
}

// override the `Next` method
func (r *SpecialResponse) Next() quest.NextInterface {
  return &quest.Next{&SpecialClient{&quest.Client{}}, r.GetRequest().GetError()}
}

func main() {
  client := &SpecialClient{&quest.Client{}}

  var value interface{}
  err := client.Get("some/path/to/resource").
    Send().
    ExpectSuccess().
    GetJSON(&value)
    Done()
  
  if err != nil {
    log.Panic(err.Error())
  }

  // so something with value
}
```
