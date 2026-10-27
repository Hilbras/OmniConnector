package storetest

import (
	"testing"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/audit"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
)

func TestMemoryOperationStore(t *testing.T) {
	OperationStore(t, func(t *testing.T) kernel.OperationStore { return kernel.NewMemoryOperationStore() })
}

func TestMemoryAuditStore(t *testing.T) {
	AuditStore(t, func(t *testing.T) audit.Store { return audit.NewMemoryStore() })
}
