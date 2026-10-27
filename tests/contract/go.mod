module github.com/hilbras/omniconnector/tests/contract

go 1.22

require (
	github.com/hilbras/omniconnector/integrations/github v0.0.0
	github.com/hilbras/omniconnector/integrations/sftp v0.0.0
	github.com/hilbras/omniconnector/integrations/ssh v0.0.0
	github.com/hilbras/omniconnector/integrations/test-integration v0.0.0
	github.com/hilbras/omniconnector/integrations/vercel v0.0.0
	github.com/hilbras/omniconnector/packages/kernel-runtime v0.0.0
	github.com/hilbras/omniconnector/packages/shared-contracts v0.0.0
)

replace (
	github.com/hilbras/omniconnector/integrations/github => ../../integrations/github
	github.com/hilbras/omniconnector/integrations/sftp => ../../integrations/sftp
	github.com/hilbras/omniconnector/integrations/ssh => ../../integrations/ssh
	github.com/hilbras/omniconnector/integrations/test-integration => ../../integrations/test-integration
	github.com/hilbras/omniconnector/integrations/vercel => ../../integrations/vercel
	github.com/hilbras/omniconnector/packages/kernel-runtime => ../../packages/kernel-runtime
	github.com/hilbras/omniconnector/packages/shared-contracts => ../../packages/shared-contracts
)
