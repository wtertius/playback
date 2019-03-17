Implemented scenarios:
- Playback for single matching type / url

```
// PLAYBACK_MODE=PlaybackOrRecord go run main.go
// It's switched off by default
// Look mode.go code for other modes

import "github.com/wtertius/playback"

// First put the playback object to context
ctx = playback.NewContext(ctx, playback.Default())

// Initialize your grpc server with playback.Interceptor
playbackMiddleware := playback.NewInterceptor(ctx)
myServer := grpc.NewServer(
    grpc.UnaryInterceptor(playbackMiddleware),
)

// Use `Random` record/playback for generated values
searchId := playback.FromContext(ctx).Random("search", uuid.New()).(uuid.UUID)

// Use HTTPTransport middleware to record/playback http requests
transport := playback.FromContext(ctx).HTTPTransport(http.DefaultTransport)
httpClient := &http.Client{
    Transport: transport,
}
httpClient.Do(request)

// Use SQLRows to record/playback sql/driver.Rows queries
rows, err := playback.FromContext(ctx).SQLRows(stmt.query, args, func() (driver.Rows, error) {
    return stmt.queryContext(ctx, args)
})

// Use SQLRows to record/playback sql/driver.Result queries
results, err := playback.FromContext(ctx).SQLResult(stmt.query, args, func() (driver.Result, error) {
    return stmt.execContext(ctx, args)
})


```

TODO:
- Comparing of two analogous requests
- Playback of previous requests
