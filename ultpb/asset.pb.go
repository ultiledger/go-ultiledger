// Code generated by protoc-gen-go. DO NOT EDIT.
// source: asset.proto

package ultpb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// enumerable asset types
type Asset_AssetTypeEnum int32

const (
	Asset_NATIVE Asset_AssetTypeEnum = 0
	Asset_CUSTOM Asset_AssetTypeEnum = 1
)

var Asset_AssetTypeEnum_name = map[int32]string{
	0: "NATIVE",
	1: "CUSTOM",
}
var Asset_AssetTypeEnum_value = map[string]int32{
	"NATIVE": 0,
	"CUSTOM": 1,
}

func (x Asset_AssetTypeEnum) String() string {
	return proto.EnumName(Asset_AssetTypeEnum_name, int32(x))
}
func (Asset_AssetTypeEnum) EnumDescriptor() ([]byte, []int) { return fileDescriptor1, []int{0, 0} }

type Asset struct {
	AssetType Asset_AssetTypeEnum `protobuf:"varint,1,opt,name=AssetType,enum=ultpb.Asset_AssetTypeEnum" json:"AssetType,omitempty"`
	// user defined asset name, name length should not be more than 4
	AssetName string `protobuf:"bytes,2,opt,name=AssetName" json:"AssetName,omitempty"`
	// signer of this asset
	Signer string `protobuf:"bytes,3,opt,name=Signer" json:"Signer,omitempty"`
	// balance of this asset
	Balance uint64 `protobuf:"varint,4,opt,name=Balance" json:"Balance,omitempty"`
}

func (m *Asset) Reset()                    { *m = Asset{} }
func (m *Asset) String() string            { return proto.CompactTextString(m) }
func (*Asset) ProtoMessage()               {}
func (*Asset) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{0} }

func (m *Asset) GetAssetType() Asset_AssetTypeEnum {
	if m != nil {
		return m.AssetType
	}
	return Asset_NATIVE
}

func (m *Asset) GetAssetName() string {
	if m != nil {
		return m.AssetName
	}
	return ""
}

func (m *Asset) GetSigner() string {
	if m != nil {
		return m.Signer
	}
	return ""
}

func (m *Asset) GetBalance() uint64 {
	if m != nil {
		return m.Balance
	}
	return 0
}

func init() {
	proto.RegisterType((*Asset)(nil), "ultpb.Asset")
	proto.RegisterEnum("ultpb.Asset_AssetTypeEnum", Asset_AssetTypeEnum_name, Asset_AssetTypeEnum_value)
}

func init() { proto.RegisterFile("asset.proto", fileDescriptor1) }

var fileDescriptor1 = []byte{
	// 171 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x4e, 0x2c, 0x2e, 0x4e,
	0x2d, 0xd1, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x2d, 0xcd, 0x29, 0x29, 0x48, 0x52, 0xda,
	0xc5, 0xc8, 0xc5, 0xea, 0x08, 0x12, 0x16, 0xb2, 0xe0, 0xe2, 0x04, 0x33, 0x42, 0x2a, 0x0b, 0x52,
	0x25, 0x18, 0x15, 0x18, 0x35, 0xf8, 0x8c, 0xa4, 0xf4, 0xc0, 0x8a, 0xf4, 0xc0, 0xe2, 0x7a, 0x70,
	0x59, 0xd7, 0xbc, 0xd2, 0xdc, 0x20, 0x84, 0x62, 0x21, 0x19, 0xa8, 0x4e, 0xbf, 0xc4, 0xdc, 0x54,
	0x09, 0x26, 0x05, 0x46, 0x0d, 0xce, 0x20, 0x84, 0x80, 0x90, 0x18, 0x17, 0x5b, 0x70, 0x66, 0x7a,
	0x5e, 0x6a, 0x91, 0x04, 0x33, 0x58, 0x0a, 0xca, 0x13, 0x92, 0xe0, 0x62, 0x77, 0x4a, 0xcc, 0x49,
	0xcc, 0x4b, 0x4e, 0x95, 0x60, 0x51, 0x60, 0xd4, 0x60, 0x09, 0x82, 0x71, 0x95, 0xd4, 0xb9, 0x78,
	0x51, 0xec, 0x12, 0xe2, 0xe2, 0x62, 0xf3, 0x73, 0x0c, 0xf1, 0x0c, 0x73, 0x15, 0x60, 0x00, 0xb1,
	0x9d, 0x43, 0x83, 0x43, 0xfc, 0x7d, 0x05, 0x18, 0x93, 0xd8, 0xc0, 0x5e, 0x31, 0x06, 0x04, 0x00,
	0x00, 0xff, 0xff, 0xca, 0xae, 0x2e, 0xf1, 0xd9, 0x00, 0x00, 0x00,
}
