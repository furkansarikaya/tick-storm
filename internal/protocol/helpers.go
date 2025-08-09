// Package protocol provides helper functions for protocol operations.
package protocol

import (
	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

// ConvertPBMessageType converts protobuf MessageType enum to protocol MessageType.
func ConvertPBMessageType(pbType pb.MessageType) MessageType {
	switch pbType {
	case pb.MessageType_MESSAGE_TYPE_AUTH:
		return MessageTypeAuth
	case pb.MessageType_MESSAGE_TYPE_SUBSCRIBE:
		return MessageTypeSubscribe
	case pb.MessageType_MESSAGE_TYPE_HEARTBEAT:
		return MessageTypeHeartbeat
	case pb.MessageType_MESSAGE_TYPE_DATA_BATCH:
		return MessageTypeDataBatch
	case pb.MessageType_MESSAGE_TYPE_ERROR:
		return MessageTypeError
	case pb.MessageType_MESSAGE_TYPE_ACK:
		return MessageTypeACK
	case pb.MessageType_MESSAGE_TYPE_PONG:
		return MessageTypePong
	default:
		return 0
	}
}

// ConvertToProtobufMessageType converts protocol MessageType to protobuf MessageType enum.
func ConvertToProtobufMessageType(msgType MessageType) pb.MessageType {
	switch msgType {
	case MessageTypeAuth:
		return pb.MessageType_MESSAGE_TYPE_AUTH
	case MessageTypeSubscribe:
		return pb.MessageType_MESSAGE_TYPE_SUBSCRIBE
	case MessageTypeHeartbeat:
		return pb.MessageType_MESSAGE_TYPE_HEARTBEAT
	case MessageTypeDataBatch:
		return pb.MessageType_MESSAGE_TYPE_DATA_BATCH
	case MessageTypeError:
		return pb.MessageType_MESSAGE_TYPE_ERROR
	case MessageTypeACK:
		return pb.MessageType_MESSAGE_TYPE_ACK
	case MessageTypePong:
		return pb.MessageType_MESSAGE_TYPE_PONG
	default:
		return pb.MessageType_MESSAGE_TYPE_UNSPECIFIED
	}
}
