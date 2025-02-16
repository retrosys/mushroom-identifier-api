module github.com/retrosys/mushroom-identifier-api

go 1.21

require (
	github.com/retrosys/mushroom-identifier-api/handlers v0.0.0-00010101000000-000000000000
	github.com/retrosys/mushroom-identifier-api/models v0.0.0-00010101000000-000000000000
	github.com/retrosys/mushroom-identifier-api/services v0.0.0-00010101000000-000000000000
	github.com/retrosys/mushroom-identifier-api/utils v0.0.0-00010101000000-000000000000
)

replace (
	github.com/retrosys/mushroom-identifier-api/handlers => ./handlers
	github.com/retrosys/mushroom-identifier-api/models => ./models
	github.com/retrosys/mushroom-identifier-api/services => ./services
	github.com/retrosys/mushroom-identifier-api/utils => ./utils
)
