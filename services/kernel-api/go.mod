module github.com/hilbras/omniconnector/services/kernel-api

go 1.22

require (
	github.com/hilbras/omniconnector/integrations/test-integration v0.0.0
	github.com/hilbras/omniconnector/packages/kernel-runtime v0.0.0
	github.com/hilbras/omniconnector/packages/shared-contracts v0.0.0
)

replace (
	github.com/hilbras/omniconnector/integrations/test-integration => ../../integrations/test-integration
	github.com/hilbras/omniconnector/packages/kernel-runtime => ../../packages/kernel-runtime
	github.com/hilbras/omniconnector/packages/shared-contracts => ../../packages/shared-contracts
)
