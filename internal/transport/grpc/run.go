package grpc

import (
	"context"
	"fmt"
	"net"

	"pet-ticket/api/gen/go/ticket/v1"
	"pet-ticket/internal/app/tickets"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Run запускает gRPC сервер на указанном адресе
func Run(listen string, svc tickets.Service, logger zerolog.Logger) (*grpc.Server, net.Listener, error) {
	if listen == "" {
		return nil, nil, nil
	}

	lc := net.ListenConfig{}
	lis, err := lc.Listen(context.Background(), "tcp", listen)
	if err != nil {
		return nil, nil, fmt.Errorf("grpc listen: %w", err)
	}

	srv := grpc.NewServer()
	ticketv1.RegisterTicketServiceServer(srv, NewServer(svc))
	reflection.Register(srv) // для grpcurl / evans

	go func() {
		logger.Info().Str("bind", listen).Msg("starting grpc server")
		if err := srv.Serve(lis); err != nil {
			logger.Error().Err(err).Msg("grpc server stopped")
		}
	}()

	return srv, lis, nil
}
