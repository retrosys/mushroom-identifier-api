module github.com/retrosys/mushroom-identifier-api/handlers

go 1.21

require (
	github.com/retrosys/mushroom-identifier-api/models v0.0.0
	github.com/retrosys/mushroom-identifier-api/services v0.0.0
)

replace (
	github.com/retrosys/mushroom-identifier-api/models => ../models
	github.com/retrosys/mushroom-identifier-api/services => ../services
)
