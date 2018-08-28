// Code generated by protoc-gen-go. DO NOT EDIT.
// source: service.proto

/*
Package rpcpb is a generated protocol buffer package.

It is generated from these files:
	service.proto

It has these top-level messages:
	HelloRequest
	HelloResponse
	SubmitTxRequest
	SubmitTxResponse
	NotifyRequest
	NotifyResponse
*/
package rpcpb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type TxStatusEnum int32

const (
	TxStatusEnum_NOTEXIST  TxStatusEnum = 0
	TxStatusEnum_REJECTED  TxStatusEnum = 1
	TxStatusEnum_ACCEPTED  TxStatusEnum = 2
	TxStatusEnum_CONFIRMED TxStatusEnum = 3
	TxStatusEnum_FAILED    TxStatusEnum = 4
)

var TxStatusEnum_name = map[int32]string{
	0: "NOTEXIST",
	1: "REJECTED",
	2: "ACCEPTED",
	3: "CONFIRMED",
	4: "FAILED",
}
var TxStatusEnum_value = map[string]int32{
	"NOTEXIST":  0,
	"REJECTED":  1,
	"ACCEPTED":  2,
	"CONFIRMED": 3,
	"FAILED":    4,
}

func (x TxStatusEnum) String() string {
	return proto.EnumName(TxStatusEnum_name, int32(x))
}
func (TxStatusEnum) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type NotifyMsgType int32

const (
	NotifyMsgType_Tx        NotifyMsgType = 0
	NotifyMsgType_STATEMENT NotifyMsgType = 1
)

var NotifyMsgType_name = map[int32]string{
	0: "Tx",
	1: "STATEMENT",
}
var NotifyMsgType_value = map[string]int32{
	"Tx":        0,
	"STATEMENT": 1,
}

func (x NotifyMsgType) String() string {
	return proto.EnumName(NotifyMsgType_name, int32(x))
}
func (NotifyMsgType) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

type HelloRequest struct {
}

func (m *HelloRequest) Reset()                    { *m = HelloRequest{} }
func (m *HelloRequest) String() string            { return proto.CompactTextString(m) }
func (*HelloRequest) ProtoMessage()               {}
func (*HelloRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type HelloResponse struct {
}

func (m *HelloResponse) Reset()                    { *m = HelloResponse{} }
func (m *HelloResponse) String() string            { return proto.CompactTextString(m) }
func (*HelloResponse) ProtoMessage()               {}
func (*HelloResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

type SubmitTxRequest struct {
	// transaction data in pb format
	Data []byte `protobuf:"bytes,1,opt,name=Data,proto3" json:"Data,omitempty"`
	// digital signature of the data signed by
	// the source account private key
	Signature string `protobuf:"bytes,2,opt,name=Signature" json:"Signature,omitempty"`
}

func (m *SubmitTxRequest) Reset()                    { *m = SubmitTxRequest{} }
func (m *SubmitTxRequest) String() string            { return proto.CompactTextString(m) }
func (*SubmitTxRequest) ProtoMessage()               {}
func (*SubmitTxRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *SubmitTxRequest) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *SubmitTxRequest) GetSignature() string {
	if m != nil {
		return m.Signature
	}
	return ""
}

type SubmitTxResponse struct {
	TxStatus TxStatusEnum `protobuf:"varint,1,opt,name=TxStatus,enum=rpcpb.TxStatusEnum" json:"TxStatus,omitempty"`
	// the transaction hash is only valid when the response
	// status is ACCEPTED or CONFIRMED
	TxHash string `protobuf:"bytes,2,opt,name=TxHash" json:"TxHash,omitempty"`
	// error message for REJECTED transaction
	ErrorMessage string `protobuf:"bytes,3,opt,name=ErrorMessage" json:"ErrorMessage,omitempty"`
}

func (m *SubmitTxResponse) Reset()                    { *m = SubmitTxResponse{} }
func (m *SubmitTxResponse) String() string            { return proto.CompactTextString(m) }
func (*SubmitTxResponse) ProtoMessage()               {}
func (*SubmitTxResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *SubmitTxResponse) GetTxStatus() TxStatusEnum {
	if m != nil {
		return m.TxStatus
	}
	return TxStatusEnum_NOTEXIST
}

func (m *SubmitTxResponse) GetTxHash() string {
	if m != nil {
		return m.TxHash
	}
	return ""
}

func (m *SubmitTxResponse) GetErrorMessage() string {
	if m != nil {
		return m.ErrorMessage
	}
	return ""
}

type NotifyRequest struct {
	// type of message
	MsgType NotifyMsgType `protobuf:"varint,1,opt,name=MsgType,enum=rpcpb.NotifyMsgType" json:"MsgType,omitempty"`
	// message payload in pb format
	Data []byte `protobuf:"bytes,2,opt,name=Data,proto3" json:"Data,omitempty"`
	// digital signature of the data signed by
	// the private key of peer node
	Signature string `protobuf:"bytes,3,opt,name=Signature" json:"Signature,omitempty"`
}

func (m *NotifyRequest) Reset()                    { *m = NotifyRequest{} }
func (m *NotifyRequest) String() string            { return proto.CompactTextString(m) }
func (*NotifyRequest) ProtoMessage()               {}
func (*NotifyRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *NotifyRequest) GetMsgType() NotifyMsgType {
	if m != nil {
		return m.MsgType
	}
	return NotifyMsgType_Tx
}

func (m *NotifyRequest) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *NotifyRequest) GetSignature() string {
	if m != nil {
		return m.Signature
	}
	return ""
}

type NotifyResponse struct {
}

func (m *NotifyResponse) Reset()                    { *m = NotifyResponse{} }
func (m *NotifyResponse) String() string            { return proto.CompactTextString(m) }
func (*NotifyResponse) ProtoMessage()               {}
func (*NotifyResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func init() {
	proto.RegisterType((*HelloRequest)(nil), "rpcpb.HelloRequest")
	proto.RegisterType((*HelloResponse)(nil), "rpcpb.HelloResponse")
	proto.RegisterType((*SubmitTxRequest)(nil), "rpcpb.SubmitTxRequest")
	proto.RegisterType((*SubmitTxResponse)(nil), "rpcpb.SubmitTxResponse")
	proto.RegisterType((*NotifyRequest)(nil), "rpcpb.NotifyRequest")
	proto.RegisterType((*NotifyResponse)(nil), "rpcpb.NotifyResponse")
	proto.RegisterEnum("rpcpb.TxStatusEnum", TxStatusEnum_name, TxStatusEnum_value)
	proto.RegisterEnum("rpcpb.NotifyMsgType", NotifyMsgType_name, NotifyMsgType_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for Node service

type NodeClient interface {
	Hello(ctx context.Context, in *HelloRequest, opts ...grpc.CallOption) (*HelloResponse, error)
	SubmitTx(ctx context.Context, in *SubmitTxRequest, opts ...grpc.CallOption) (*SubmitTxResponse, error)
	Notify(ctx context.Context, in *NotifyRequest, opts ...grpc.CallOption) (*NotifyResponse, error)
}

type nodeClient struct {
	cc *grpc.ClientConn
}

func NewNodeClient(cc *grpc.ClientConn) NodeClient {
	return &nodeClient{cc}
}

func (c *nodeClient) Hello(ctx context.Context, in *HelloRequest, opts ...grpc.CallOption) (*HelloResponse, error) {
	out := new(HelloResponse)
	err := grpc.Invoke(ctx, "/rpcpb.Node/Hello", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) SubmitTx(ctx context.Context, in *SubmitTxRequest, opts ...grpc.CallOption) (*SubmitTxResponse, error) {
	out := new(SubmitTxResponse)
	err := grpc.Invoke(ctx, "/rpcpb.Node/SubmitTx", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) Notify(ctx context.Context, in *NotifyRequest, opts ...grpc.CallOption) (*NotifyResponse, error) {
	out := new(NotifyResponse)
	err := grpc.Invoke(ctx, "/rpcpb.Node/Notify", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Node service

type NodeServer interface {
	Hello(context.Context, *HelloRequest) (*HelloResponse, error)
	SubmitTx(context.Context, *SubmitTxRequest) (*SubmitTxResponse, error)
	Notify(context.Context, *NotifyRequest) (*NotifyResponse, error)
}

func RegisterNodeServer(s *grpc.Server, srv NodeServer) {
	s.RegisterService(&_Node_serviceDesc, srv)
}

func _Node_Hello_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HelloRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Hello(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rpcpb.Node/Hello",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Hello(ctx, req.(*HelloRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_SubmitTx_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SubmitTxRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).SubmitTx(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rpcpb.Node/SubmitTx",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).SubmitTx(ctx, req.(*SubmitTxRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_Notify_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NotifyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Notify(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rpcpb.Node/Notify",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Notify(ctx, req.(*NotifyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Node_serviceDesc = grpc.ServiceDesc{
	ServiceName: "rpcpb.Node",
	HandlerType: (*NodeServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Hello",
			Handler:    _Node_Hello_Handler,
		},
		{
			MethodName: "SubmitTx",
			Handler:    _Node_SubmitTx_Handler,
		},
		{
			MethodName: "Notify",
			Handler:    _Node_Notify_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "service.proto",
}

func init() { proto.RegisterFile("service.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 394 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x52, 0x4d, 0xaf, 0xd2, 0x40,
	0x14, 0xa5, 0x85, 0x57, 0xe1, 0xa6, 0xe5, 0x4d, 0xc6, 0xe7, 0x93, 0x10, 0x17, 0xa4, 0x0b, 0x43,
	0x58, 0xd4, 0x04, 0x4d, 0x5c, 0xb9, 0x20, 0xed, 0x10, 0x30, 0xb6, 0x98, 0x76, 0x16, 0x6e, 0x0b,
	0x8e, 0xd8, 0x04, 0x68, 0xe9, 0x4c, 0x4d, 0x59, 0xf9, 0xa7, 0xfc, 0x81, 0xa6, 0xed, 0x0c, 0x5f,
	0xc6, 0xdd, 0x9c, 0x33, 0xe7, 0x7e, 0x9c, 0x7b, 0x2f, 0x58, 0x9c, 0xe5, 0xbf, 0x92, 0x0d, 0x73,
	0xb2, 0x3c, 0x15, 0x29, 0x7e, 0xc8, 0xb3, 0x4d, 0xb6, 0xb6, 0xfb, 0x60, 0x2e, 0xd8, 0x6e, 0x97,
	0x86, 0xec, 0x58, 0x30, 0x2e, 0xec, 0x47, 0xb0, 0x24, 0xe6, 0x59, 0x7a, 0xe0, 0xcc, 0x76, 0xe1,
	0x31, 0x2a, 0xd6, 0xfb, 0x44, 0xd0, 0x52, 0x6a, 0x30, 0x86, 0x8e, 0x17, 0x8b, 0x78, 0xa0, 0x8d,
	0xb4, 0xb1, 0x19, 0xd6, 0x6f, 0xfc, 0x06, 0x7a, 0x51, 0xb2, 0x3d, 0xc4, 0xa2, 0xc8, 0xd9, 0x40,
	0x1f, 0x69, 0xe3, 0x5e, 0x78, 0x21, 0xec, 0xdf, 0x80, 0x2e, 0x49, 0x9a, 0xc4, 0xf8, 0x1d, 0x74,
	0x69, 0x19, 0x89, 0x58, 0x14, 0xbc, 0xce, 0xd4, 0x9f, 0xbe, 0x74, 0xea, 0x9e, 0x1c, 0x45, 0x93,
	0x43, 0xb1, 0x0f, 0xcf, 0x22, 0xfc, 0x0c, 0x06, 0x2d, 0x17, 0x31, 0xff, 0x29, 0xf3, 0x4b, 0x84,
	0x6d, 0x30, 0x49, 0x9e, 0xa7, 0xb9, 0xcf, 0x38, 0x8f, 0xb7, 0x6c, 0xd0, 0xae, 0x7f, 0x6f, 0x38,
	0xfb, 0x08, 0x56, 0x90, 0x8a, 0xe4, 0xc7, 0x49, 0x79, 0x70, 0xe0, 0x85, 0xcf, 0xb7, 0xf4, 0x94,
	0x31, 0x59, 0xfc, 0x49, 0x16, 0x6f, 0x64, 0xf2, 0x2f, 0x54, 0xa2, 0xb3, 0x67, 0xfd, 0x7f, 0x9e,
	0xdb, 0xf7, 0x9e, 0x11, 0xf4, 0x55, 0xc9, 0xc6, 0xf1, 0x24, 0x02, 0xf3, 0xda, 0x1a, 0x36, 0xa1,
	0x1b, 0xac, 0x28, 0xf9, 0xb6, 0x8c, 0x28, 0x6a, 0x55, 0x28, 0x24, 0x9f, 0x89, 0x4b, 0x89, 0x87,
	0xb4, 0x0a, 0xcd, 0x5c, 0x97, 0x7c, 0xad, 0x90, 0x8e, 0x2d, 0xe8, 0xb9, 0xab, 0x60, 0xbe, 0x0c,
	0x7d, 0xe2, 0xa1, 0x36, 0x06, 0x30, 0xe6, 0xb3, 0xe5, 0x17, 0xe2, 0xa1, 0xce, 0xe4, 0xad, 0x72,
	0xa6, 0x3a, 0x35, 0x40, 0xa7, 0x25, 0x6a, 0x55, 0x31, 0x11, 0x9d, 0x51, 0xe2, 0x93, 0x80, 0x22,
	0x6d, 0xfa, 0x47, 0x83, 0x4e, 0x90, 0x7e, 0x67, 0xf8, 0x03, 0x3c, 0xd4, 0x1b, 0xc6, 0x6a, 0xdc,
	0xd7, 0xfb, 0x1f, 0x3e, 0xdd, 0x92, 0xf2, 0x08, 0x5a, 0xf8, 0x13, 0x74, 0xd5, 0x06, 0xf1, 0xb3,
	0xd4, 0xdc, 0xdd, 0xc5, 0xf0, 0xf5, 0x3f, 0xfc, 0x39, 0xfc, 0x23, 0x18, 0x4d, 0x97, 0xf8, 0x76,
	0xce, 0x2a, 0xf4, 0xd5, 0x1d, 0xab, 0x02, 0xd7, 0x46, 0x7d, 0xad, 0xef, 0xff, 0x06, 0x00, 0x00,
	0xff, 0xff, 0xea, 0xfe, 0xae, 0x99, 0xbe, 0x02, 0x00, 0x00,
}