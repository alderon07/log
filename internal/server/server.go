package server

import (
	"context"

	api "github.com/alderon07/log/api/v1"
	"google.golang.org/grpc"
)

// Keep log storage separate from the gRPC service implementation.
type CommitLog interface {
	Append(*api.Record) (uint64, error)
	Read(uint64) (*api.Record, error)
}

type Config struct {
	CommitLog CommitLog
}

var _ api.LogServer = (*grpcLogService)(nil)

func NewGRPCServer(config *Config) (*grpc.Server, error) {
	gsrv := grpc.NewServer()
	logService, err := newGrpcLogService(config)
	if err != nil {
		return nil, err
	}

	// Register the log service so the server can handle its RPCs.
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

	// Return the offset assigned by the log.
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
		// Receive and process one record at a time.
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		res, err := s.Produce(stream.Context(), req)
		if err != nil {
			return err
		}

		// Send the offset assigned to this record back to the client.
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
			// Read the next record, retrying if its offset is not available yet.
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

			// Advance so the next iteration reads the following record.
			req.Offset++
		}
	}
}
