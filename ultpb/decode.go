package ultpb

import (
	"github.com/golang/protobuf/proto"
)

// decode pb message to Transaction
func DecodeTransaction(b []byte) (*Transaction, error) {
	tx := &Transaction{}
	if err := proto.Unmarshal(b, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

// decode pb message to CreateAccountOp
func DecodeCreateAccountOp(b []byte) (*CreateAccountOp, error) {
	op := &CreateAccountOp{}
	if err := proto.Unmarshal(b, op); err != nil {
		return nil, err
	}
	return op, nil
}

// decode pb message to PaymentOp
func DecodePaymentOp(b []byte) (*PaymentOp, error) {
	op := &PaymentOp{}
	if err := proto.Unmarshal(b, op); err != nil {
		return nil, err
	}
	return op, nil
}
