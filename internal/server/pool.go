package server

import (
	"sync"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

// ObjectPools contains all object pools for memory optimization
type ObjectPools struct {
	// Frame pools
	framePool     sync.Pool
	frameDataPool sync.Pool
	
	// Protobuf message pools
	authRequestPool      sync.Pool
	subscribeRequestPool sync.Pool
	heartbeatRequestPool sync.Pool
	tickPool            sync.Pool
	dataBatchPool       sync.Pool
	errorResponsePool   sync.Pool
	ackResponsePool     sync.Pool
	heartbeatRespPool   sync.Pool
	
	// Buffer pools
	readBufferPool  sync.Pool
	writeBufferPool sync.Pool
}

// NewObjectPools creates and initializes all object pools
func NewObjectPools() *ObjectPools {
	pools := &ObjectPools{}
	
	// Frame pools
	pools.framePool = sync.Pool{
		New: func() interface{} {
			return &protocol.Frame{}
		},
	}
	
	pools.frameDataPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 1024) // 1KB initial capacity
		},
	}
	
	// Protobuf message pools
	pools.authRequestPool = sync.Pool{
		New: func() interface{} {
			return &pb.AuthRequest{}
		},
	}
	
	pools.subscribeRequestPool = sync.Pool{
		New: func() interface{} {
			return &pb.SubscribeRequest{}
		},
	}
	
	pools.heartbeatRequestPool = sync.Pool{
		New: func() interface{} {
			return &pb.HeartbeatRequest{}
		},
	}
	
	pools.tickPool = sync.Pool{
		New: func() interface{} {
			return &pb.Tick{}
		},
	}
	
	pools.dataBatchPool = sync.Pool{
		New: func() interface{} {
			return &pb.DataBatch{
				Ticks: make([]*pb.Tick, 0, 100), // Pre-allocate for 100 ticks
			}
		},
	}
	
	pools.errorResponsePool = sync.Pool{
		New: func() interface{} {
			return &pb.ErrorResponse{}
		},
	}
	
	pools.ackResponsePool = sync.Pool{
		New: func() interface{} {
			return &pb.AckResponse{}
		},
	}
	
	pools.heartbeatRespPool = sync.Pool{
		New: func() interface{} {
			return &pb.HeartbeatResponse{}
		},
	}
	
	// Buffer pools
	pools.readBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 4096) // 4KB read buffer
		},
	}
	
	pools.writeBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 4096) // 4KB write buffer
		},
	}
	
	return pools
}

// Frame pool methods
func (p *ObjectPools) GetFrame() *protocol.Frame {
	frame := p.framePool.Get().(*protocol.Frame)
	// Reset frame
	frame.Magic = [2]byte{}
	frame.Version = 0
	frame.Type = 0
	frame.Length = 0
	frame.Payload = nil
	frame.CRC = 0
	return frame
}

func (p *ObjectPools) PutFrame(frame *protocol.Frame) {
	if frame != nil {
		p.framePool.Put(frame)
	}
}

func (p *ObjectPools) GetFrameData() []byte {
	return p.frameDataPool.Get().([]byte)[:0] // Reset slice length
}

func (p *ObjectPools) PutFrameData(data []byte) {
	if cap(data) <= 8192 { // Only pool reasonably sized buffers
		p.frameDataPool.Put(data)
	}
}

// Protobuf message pool methods
func (p *ObjectPools) GetAuthRequest() *pb.AuthRequest {
	req := p.authRequestPool.Get().(*pb.AuthRequest)
	req.Reset()
	return req
}

func (p *ObjectPools) PutAuthRequest(req *pb.AuthRequest) {
	if req != nil {
		p.authRequestPool.Put(req)
	}
}

func (p *ObjectPools) GetSubscribeRequest() *pb.SubscribeRequest {
	req := p.subscribeRequestPool.Get().(*pb.SubscribeRequest)
	req.Reset()
	return req
}

func (p *ObjectPools) PutSubscribeRequest(req *pb.SubscribeRequest) {
	if req != nil {
		p.subscribeRequestPool.Put(req)
	}
}

func (p *ObjectPools) GetHeartbeatRequest() *pb.HeartbeatRequest {
	req := p.heartbeatRequestPool.Get().(*pb.HeartbeatRequest)
	req.Reset()
	return req
}

func (p *ObjectPools) PutHeartbeatRequest(req *pb.HeartbeatRequest) {
	if req != nil {
		p.heartbeatRequestPool.Put(req)
	}
}

func (p *ObjectPools) GetTick() *pb.Tick {
	tick := p.tickPool.Get().(*pb.Tick)
	tick.Reset()
	return tick
}

func (p *ObjectPools) PutTick(tick *pb.Tick) {
	if tick != nil {
		p.tickPool.Put(tick)
	}
}

func (p *ObjectPools) GetDataBatch() *pb.DataBatch {
	batch := p.dataBatchPool.Get().(*pb.DataBatch)
	batch.Reset()
	// Keep the pre-allocated slice but reset length
	if batch.Ticks != nil {
		batch.Ticks = batch.Ticks[:0]
	} else {
		batch.Ticks = make([]*pb.Tick, 0, 100)
	}
	return batch
}

func (p *ObjectPools) PutDataBatch(batch *pb.DataBatch) {
	if batch != nil {
		// Don't pool batches with too many ticks to avoid memory bloat
		if len(batch.Ticks) <= 200 {
			p.dataBatchPool.Put(batch)
		}
	}
}

func (p *ObjectPools) GetErrorResponse() *pb.ErrorResponse {
	resp := p.errorResponsePool.Get().(*pb.ErrorResponse)
	resp.Reset()
	return resp
}

func (p *ObjectPools) PutErrorResponse(resp *pb.ErrorResponse) {
	if resp != nil {
		p.errorResponsePool.Put(resp)
	}
}

func (p *ObjectPools) GetAckResponse() *pb.AckResponse {
	resp := p.ackResponsePool.Get().(*pb.AckResponse)
	resp.Reset()
	return resp
}

func (p *ObjectPools) PutAckResponse(resp *pb.AckResponse) {
	if resp != nil {
		p.ackResponsePool.Put(resp)
	}
}

func (p *ObjectPools) GetHeartbeatResponse() *pb.HeartbeatResponse {
	resp := p.heartbeatRespPool.Get().(*pb.HeartbeatResponse)
	resp.Reset()
	return resp
}

func (p *ObjectPools) PutHeartbeatResponse(resp *pb.HeartbeatResponse) {
	if resp != nil {
		p.heartbeatRespPool.Put(resp)
	}
}

// Buffer pool methods
func (p *ObjectPools) GetReadBuffer() []byte {
	return p.readBufferPool.Get().([]byte)
}

func (p *ObjectPools) PutReadBuffer(buf []byte) {
	if cap(buf) == 4096 { // Only pool buffers of expected size
		p.readBufferPool.Put(buf)
	}
}

func (p *ObjectPools) GetWriteBuffer() []byte {
	return p.writeBufferPool.Get().([]byte)[:0] // Reset slice length
}

func (p *ObjectPools) PutWriteBuffer(buf []byte) {
	if cap(buf) <= 8192 { // Only pool reasonably sized buffers
		p.writeBufferPool.Put(buf)
	}
}

// Global pools instance
var globalPools = NewObjectPools()

// GetGlobalPools returns the global object pools instance
func GetGlobalPools() *ObjectPools {
	return globalPools
}
