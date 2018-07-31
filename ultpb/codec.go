package ultpb

import (
	"github.com/golang/protobuf/proto"
)

// encode pb message to bytes
func Encode(msg proto.Message) ([]byte, error) {
	b, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return b, nil
}

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
