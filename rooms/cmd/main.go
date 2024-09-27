package main

import (
	"Hotelium/rooms/internal/service"
	pb "Hotelium/rooms/pkg/rooms"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
)

func main() {
	db, err := sqlx.Connect("postgres", "user=navidnazem dbname=hotelium sslmode=disable password= host=localhost")
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()
	// Creiamo un listener TCP che ascolterà le richieste in arrivo sulla porta specificata.
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Creazione del server gRPC, che si occuperà di gestire le richieste in arrivo e di smistarle
	grcpServer := grpc.NewServer()

	// Creazione del servizio roomService, che implementa i metodi definiti nel file rooms.proto.
	// Ritorna un'istanza del server gRPC che implementa i metodi previsti.
	roomService := service.NewRoomServiceServer(db)

	// Registrazione del servizio RoomService sul server gRPC
	pb.RegisterRoomServiceServer(grcpServer, roomService)

	// Abilitazione della reflection
	reflection.Register(grcpServer)

	log.Printf("Server listening at %v", lis.Addr())
	if err := grcpServer.Serve(lis); err != nil {
		log.Fatalf("Failed to server %v", err)
	}
}
