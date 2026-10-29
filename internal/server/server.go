package server

import (
	"context"

	api "github.com/alderon07/log/api/v1"
	"google.golang.org/grpc"
)

// using this to abstract the log implementation from the grpc server implementation. This allows us to swap out the log implementation with a different one if we want to, without having to change the grpc server implementation.
type CommitLog interface {
	Append(*api.Record) (uint64, error)
	Read(uint64) (*api.Record, error) 
}

type Config struct {
	CommitLog CommitLog
}

var _ api.LogServer = (*grpcLogService)(nil)

func NewGRPCServer(config *Config) (*grpc.Server, error){
	gsrv := grpc.NewServer()
	logService, err := newGrpcLogService(config)
	if err != nil {
		return nil , err
	}

	api.RegisterLogServer(gsrv, logService)
	return gsrv, nil
}


type grpcLogService struct {
	api.UnimplementedLogServer
	*Config
}

func newGrpcLogService(config *Config) (srv *grpcLogService, err error) {
	srv = &grpcLogService{Config: config}
	return srv, nil
}

func (s *grpcLogService) Produce(ctx context.Context, req *api.ProduceRequest) (*api.ProduceResponse, error) {
	offset, err := s.CommitLog.Append(req.Record)
	if err != nil {
		return nil, err
	}

	return &api.ProduceResponse{Offset: offset}, err
}

func (s *grpcLogService) Consume(ctx context.Context, req *api.ConsumeRequest) (*api.ConsumeResponse, error) {
	record, err := s.CommitLog.Read(req.Offset)
	if err != nil {
		return nil, err
	}

	return &api.ConsumeResponse{Record: record}, err
}

func (s *grpcLogService) ProduceStream(stream api.Log_ProduceStreamServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		res, err := s.Produce(stream.Context(), req)
		if err != nil {
			return err
		}

		if err = stream.Send(res); err != nil {
			return err
		}
	}
}

func (s *grpcLogService) CosumeStream(req *api.ConsumeRequest, stream api.Log_ConsumeStreamServer) error {
	for {
		select {
		case <-stream.Context().Done():
			return nil

		default:
			res, err := s.Consume(stream.Context(), req)
			switch err := err.(type) {
			case nil:
			case api.ErrOffsetOutOfRange:
				continue
			default:
				return err
			}

			if err = stream.Send(res); err != nil {
				return err
			}
			req.Offset++
		}
	}
}

// func toProtoRecord(r Record) *api.Record {
// 	return &api.Record{
// 		Value:  r.Value,
// 		Offset: r.Offset,
// 	}
// }

// func toLocalRecord(r *api.Record) Record {
// 	if r == nil {
// 		return Record{}
// 	}
// 	return Record{
// 		Value:  r.Value,
// 		Offset: r.Offset,
// 	}
// }
