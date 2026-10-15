package server

import (
	"context"

	api "github.com/alderon07/log/api/v1"
	"google.golang.org/grpc"
)

type Config struct {
	Log Log
}

var _ api.LogServer = (*grpcServer)(nil)

type grpcServer struct {
	api.UnimplementedLogServer
	*Config
}

func newGrpcServer(config *Config) (srv *grpcServer, err error){
	srv = &grpcServer{Config: config}
	return srv, nil
}

func (s *grpcServer) Produce(ctx context.Context, req *api.ProduceRequest) (*api.ProduceResponse, error){
	offset, err := s.Log.Append(toLocalRecord(req.Record))
	if err != nil {
		return nil, err
	}

	return &api.ProduceResponse{Offset: offset}, err
}

func (s *grpcServer) Consume(ctx context.Context, req *api.ConsumeRequest) (*api.ConsumeResponse, error){
	record, err := s.Log.Read(req.Offset)
	if err != nil {
		return nil, err
	}

	return &api.ConsumeResponse{Record: toProtoRecord(record)}, err
}

func toProtoRecord(r Record) *api.Record {
    return &api.Record{
        Value:  r.Value,
        Offset: r.Offset,
    }
}

func toLocalRecord(r *api.Record) Record {
    if r == nil {
        return Record{}
    }
    return Record{
        Value:  r.Value,
        Offset: r.Offset,
    }
}