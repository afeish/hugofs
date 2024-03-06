// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: meta/event.proto

package meta

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Event_Type int32

const (
	Event_INVALID             Event_Type = 0
	Event_NEW_FILE_INODE      Event_Type = 1
	Event_NEW_DIR_INODE       Event_Type = 2
	Event_REMOVE_FILE_INODE   Event_Type = 3
	Event_REMOVE_DIR_INODE    Event_Type = 4
	Event_REMOVE_DIR_CHILDREN Event_Type = 5
	Event_UPDATE_ATTR         Event_Type = 6
)

// Enum value maps for Event_Type.
var (
	Event_Type_name = map[int32]string{
		0: "INVALID",
		1: "NEW_FILE_INODE",
		2: "NEW_DIR_INODE",
		3: "REMOVE_FILE_INODE",
		4: "REMOVE_DIR_INODE",
		5: "REMOVE_DIR_CHILDREN",
		6: "UPDATE_ATTR",
	}
	Event_Type_value = map[string]int32{
		"INVALID":             0,
		"NEW_FILE_INODE":      1,
		"NEW_DIR_INODE":       2,
		"REMOVE_FILE_INODE":   3,
		"REMOVE_DIR_INODE":    4,
		"REMOVE_DIR_CHILDREN": 5,
		"UPDATE_ATTR":         6,
	}
)

func (x Event_Type) Enum() *Event_Type {
	p := new(Event_Type)
	*p = x
	return p
}

func (x Event_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Event_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_meta_event_proto_enumTypes[0].Descriptor()
}

func (Event_Type) Type() protoreflect.EnumType {
	return &file_meta_event_proto_enumTypes[0]
}

func (x Event_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Event_Type.Descriptor instead.
func (Event_Type) EnumDescriptor() ([]byte, []int) {
	return file_meta_event_proto_rawDescGZIP(), []int{0, 0}
}

type Event struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// id is the unique id of event and it is monotonic increasing.
	Id int64 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// type is the type of event.
	Type Event_Type `protobuf:"varint,2,opt,name=type,proto3,enum=hugo.v1.meta.Event_Type" json:"type,omitempty"`
	// parent_ino represents the parent inode of the event.
	ParentIno uint64 `protobuf:"varint,3,opt,name=parent_ino,json=parentIno,proto3" json:"parent_ino,omitempty"`
	// cur_ino represents the current inode of the event.
	CurIno uint64 `protobuf:"varint,4,opt,name=cur_ino,json=curIno,proto3" json:"cur_ino,omitempty"`
	// name represents the name of the event.
	Name string `protobuf:"bytes,5,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Event) Reset() {
	*x = Event{}
	if protoimpl.UnsafeEnabled {
		mi := &file_meta_event_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Event) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Event) ProtoMessage() {}

func (x *Event) ProtoReflect() protoreflect.Message {
	mi := &file_meta_event_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Event.ProtoReflect.Descriptor instead.
func (*Event) Descriptor() ([]byte, []int) {
	return file_meta_event_proto_rawDescGZIP(), []int{0}
}

func (x *Event) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Event) GetType() Event_Type {
	if x != nil {
		return x.Type
	}
	return Event_INVALID
}

func (x *Event) GetParentIno() uint64 {
	if x != nil {
		return x.ParentIno
	}
	return 0
}

func (x *Event) GetCurIno() uint64 {
	if x != nil {
		return x.CurIno
	}
	return 0
}

func (x *Event) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

// PullLatest is used for pulling latest events.
type PullLatest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PullLatest) Reset() {
	*x = PullLatest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_meta_event_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PullLatest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PullLatest) ProtoMessage() {}

func (x *PullLatest) ProtoReflect() protoreflect.Message {
	mi := &file_meta_event_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PullLatest.ProtoReflect.Descriptor instead.
func (*PullLatest) Descriptor() ([]byte, []int) {
	return file_meta_event_proto_rawDescGZIP(), []int{1}
}

type PullLatest_Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CurrentEventId int64 `protobuf:"varint,1,opt,name=current_event_id,json=currentEventId,proto3" json:"current_event_id,omitempty"`
}

func (x *PullLatest_Request) Reset() {
	*x = PullLatest_Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_meta_event_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PullLatest_Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PullLatest_Request) ProtoMessage() {}

func (x *PullLatest_Request) ProtoReflect() protoreflect.Message {
	mi := &file_meta_event_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PullLatest_Request.ProtoReflect.Descriptor instead.
func (*PullLatest_Request) Descriptor() ([]byte, []int) {
	return file_meta_event_proto_rawDescGZIP(), []int{1, 0}
}

func (x *PullLatest_Request) GetCurrentEventId() int64 {
	if x != nil {
		return x.CurrentEventId
	}
	return 0
}

type PullLatest_Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Events []*Event `protobuf:"bytes,1,rep,name=events,proto3" json:"events,omitempty"`
}

func (x *PullLatest_Response) Reset() {
	*x = PullLatest_Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_meta_event_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PullLatest_Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PullLatest_Response) ProtoMessage() {}

func (x *PullLatest_Response) ProtoReflect() protoreflect.Message {
	mi := &file_meta_event_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PullLatest_Response.ProtoReflect.Descriptor instead.
func (*PullLatest_Response) Descriptor() ([]byte, []int) {
	return file_meta_event_proto_rawDescGZIP(), []int{1, 1}
}

func (x *PullLatest_Response) GetEvents() []*Event {
	if x != nil {
		return x.Events
	}
	return nil
}

var File_meta_event_proto protoreflect.FileDescriptor

var file_meta_event_proto_rawDesc = []byte{
	0x0a, 0x10, 0x6d, 0x65, 0x74, 0x61, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x0c, 0x67, 0x61, 0x69, 0x61, 0x2e, 0x76, 0x31, 0x2e, 0x6d, 0x65, 0x74, 0x61,
	0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0xa5, 0x02, 0x0a, 0x05, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x69, 0x64, 0x12, 0x2c, 0x0a, 0x04, 0x74,
	0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x18, 0x2e, 0x67, 0x61, 0x69, 0x61,
	0x2e, 0x76, 0x31, 0x2e, 0x6d, 0x65, 0x74, 0x61, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x2e, 0x54,
	0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x70, 0x61, 0x72,
	0x65, 0x6e, 0x74, 0x5f, 0x69, 0x6e, 0x6f, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x70,
	0x61, 0x72, 0x65, 0x6e, 0x74, 0x49, 0x6e, 0x6f, 0x12, 0x17, 0x0a, 0x07, 0x63, 0x75, 0x72, 0x5f,
	0x69, 0x6e, 0x6f, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06, 0x63, 0x75, 0x72, 0x49, 0x6e,
	0x6f, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x91, 0x01, 0x0a, 0x04, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0b,
	0x0a, 0x07, 0x49, 0x4e, 0x56, 0x41, 0x4c, 0x49, 0x44, 0x10, 0x00, 0x12, 0x12, 0x0a, 0x0e, 0x4e,
	0x45, 0x57, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x5f, 0x49, 0x4e, 0x4f, 0x44, 0x45, 0x10, 0x01, 0x12,
	0x11, 0x0a, 0x0d, 0x4e, 0x45, 0x57, 0x5f, 0x44, 0x49, 0x52, 0x5f, 0x49, 0x4e, 0x4f, 0x44, 0x45,
	0x10, 0x02, 0x12, 0x15, 0x0a, 0x11, 0x52, 0x45, 0x4d, 0x4f, 0x56, 0x45, 0x5f, 0x46, 0x49, 0x4c,
	0x45, 0x5f, 0x49, 0x4e, 0x4f, 0x44, 0x45, 0x10, 0x03, 0x12, 0x14, 0x0a, 0x10, 0x52, 0x45, 0x4d,
	0x4f, 0x56, 0x45, 0x5f, 0x44, 0x49, 0x52, 0x5f, 0x49, 0x4e, 0x4f, 0x44, 0x45, 0x10, 0x04, 0x12,
	0x17, 0x0a, 0x13, 0x52, 0x45, 0x4d, 0x4f, 0x56, 0x45, 0x5f, 0x44, 0x49, 0x52, 0x5f, 0x43, 0x48,
	0x49, 0x4c, 0x44, 0x52, 0x45, 0x4e, 0x10, 0x05, 0x12, 0x0f, 0x0a, 0x0b, 0x55, 0x50, 0x44, 0x41,
	0x54, 0x45, 0x5f, 0x41, 0x54, 0x54, 0x52, 0x10, 0x06, 0x22, 0x7a, 0x0a, 0x0a, 0x50, 0x75, 0x6c,
	0x6c, 0x4c, 0x61, 0x74, 0x65, 0x73, 0x74, 0x1a, 0x33, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x28, 0x0a, 0x10, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x65, 0x76,
	0x65, 0x6e, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0e, 0x63, 0x75,
	0x72, 0x72, 0x65, 0x6e, 0x74, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x49, 0x64, 0x1a, 0x37, 0x0a, 0x08,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2b, 0x0a, 0x06, 0x65, 0x76, 0x65, 0x6e,
	0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x67, 0x61, 0x69, 0x61, 0x2e,
	0x76, 0x31, 0x2e, 0x6d, 0x65, 0x74, 0x61, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x52, 0x06, 0x65,
	0x76, 0x65, 0x6e, 0x74, 0x73, 0x32, 0x63, 0x0a, 0x0c, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x53, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x53, 0x0a, 0x0a, 0x50, 0x75, 0x6c, 0x6c, 0x4c, 0x61, 0x74,
	0x65, 0x73, 0x74, 0x12, 0x20, 0x2e, 0x67, 0x61, 0x69, 0x61, 0x2e, 0x76, 0x31, 0x2e, 0x6d, 0x65,
	0x74, 0x61, 0x2e, 0x50, 0x75, 0x6c, 0x6c, 0x4c, 0x61, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x21, 0x2e, 0x67, 0x61, 0x69, 0x61, 0x2e, 0x76, 0x31, 0x2e,
	0x6d, 0x65, 0x74, 0x61, 0x2e, 0x50, 0x75, 0x6c, 0x6c, 0x4c, 0x61, 0x74, 0x65, 0x73, 0x74, 0x2e,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x2e, 0x5a, 0x2c, 0x67, 0x69,
	0x74, 0x2e, 0x66, 0x61, 0x73, 0x74, 0x6f, 0x6e, 0x65, 0x74, 0x65, 0x63, 0x68, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x66, 0x61, 0x73, 0x74, 0x6f, 0x6e, 0x65, 0x2f, 0x67, 0x61, 0x69, 0x61, 0x2f, 0x72,
	0x70, 0x63, 0x2f, 0x70, 0x62, 0x2f, 0x6d, 0x65, 0x74, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_meta_event_proto_rawDescOnce sync.Once
	file_meta_event_proto_rawDescData = file_meta_event_proto_rawDesc
)

func file_meta_event_proto_rawDescGZIP() []byte {
	file_meta_event_proto_rawDescOnce.Do(func() {
		file_meta_event_proto_rawDescData = protoimpl.X.CompressGZIP(file_meta_event_proto_rawDescData)
	})
	return file_meta_event_proto_rawDescData
}

var file_meta_event_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_meta_event_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_meta_event_proto_goTypes = []interface{}{
	(Event_Type)(0),             // 0: hugo.v1.meta.Event.Type
	(*Event)(nil),               // 1: hugo.v1.meta.Event
	(*PullLatest)(nil),          // 2: hugo.v1.meta.PullLatest
	(*PullLatest_Request)(nil),  // 3: hugo.v1.meta.PullLatest.Request
	(*PullLatest_Response)(nil), // 4: hugo.v1.meta.PullLatest.Response
}
var file_meta_event_proto_depIdxs = []int32{
	0, // 0: hugo.v1.meta.Event.type:type_name -> hugo.v1.meta.Event.Type
	1, // 1: hugo.v1.meta.PullLatest.Response.events:type_name -> hugo.v1.meta.Event
	3, // 2: hugo.v1.meta.EventService.PullLatest:input_type -> hugo.v1.meta.PullLatest.Request
	4, // 3: hugo.v1.meta.EventService.PullLatest:output_type -> hugo.v1.meta.PullLatest.Response
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_meta_event_proto_init() }
func file_meta_event_proto_init() {
	if File_meta_event_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_meta_event_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Event); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_meta_event_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PullLatest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_meta_event_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PullLatest_Request); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_meta_event_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PullLatest_Response); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_meta_event_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_meta_event_proto_goTypes,
		DependencyIndexes: file_meta_event_proto_depIdxs,
		EnumInfos:         file_meta_event_proto_enumTypes,
		MessageInfos:      file_meta_event_proto_msgTypes,
	}.Build()
	File_meta_event_proto = out.File
	file_meta_event_proto_rawDesc = nil
	file_meta_event_proto_goTypes = nil
	file_meta_event_proto_depIdxs = nil
}
