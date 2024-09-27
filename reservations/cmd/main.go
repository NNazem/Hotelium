package main

import (
	"Hotelium/reservations/internal/router"
	"Hotelium/reservations/internal/service"
	pb "Hotelium/reservations/pkg/reservations"
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"net/http"
)

func main() {
	db, err := sqlx.Connect("postgres", "user=navidnazem dbname=hotelium sslmode=disable password= host=localhost")

	if err != nil {
		log.Fatalln(err)
	}

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("Failed to listen %v", err)
	}

	grpcServer := grpc.NewServer()
	reservationService := service.NewReservationServer(db)
	pb.RegisterReservationServiceServer(grpcServer, reservationService)
	reflection.Register(grpcServer)

	ctx := context.Background()
	gatewayMux := runtime.NewServeMux()
	err = pb.RegisterReservationServiceHandlerServer(ctx, gatewayMux, reservationService)

	if err != nil {
		log.Fatalf("Failed to register gRPC gateway: %v", err)
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC server: %v", err)
		}
	}()

	r := router.NewRouter()
	r.AddRoute(router.RouteConfig{
		Path:    "/{any:.*}",
		Handler: gatewayMux,
	})

	log.Println("HTTP server listening at :8080")
	if err := http.ListenAndServe(":8080", r.Mux()); err != nil {
		log.Fatalf("Failed to serve HTTP: %v", err)
	}
}
