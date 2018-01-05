package event

import (
	"github.com/Ontology/common"
	"github.com/Ontology/vm/neovm/types"
)

type NotifyEventArgs struct {
	Container common.Uint256
	CodeHash  common.Uint160
	State     types.StackItemInterface
}

