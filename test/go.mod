module github.com/wtertius/playback/test

require (
	cloud.google.com/go v0.41.0
	github.com/DATA-DOG/go-sqlmock v1.3.3
	github.com/stretchr/testify v1.3.0
	github.com/wtertius/playback v0.0.0
	google.golang.org/grpc v1.22.0
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/wtertius/playback => ../
