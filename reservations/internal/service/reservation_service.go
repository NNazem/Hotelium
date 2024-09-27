package service

import (
	"Hotelium/reservations/pkg/reservations"
	roomspb "Hotelium/reservations/pkg/rooms"
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
)

type ReservationServiceServer struct {
	reservations.UnimplementedReservationServiceServer
	db *sqlx.DB
}

func ensureReservationTableExists(db *sqlx.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS reservations(
	    id SERIAL PRIMARY KEY,
	    roomId NUMERIC,
		startingDate DATE,
		endDate DATE
	    )
`
	_, err := db.Exec(query)

	return err
}

func NewReservationServer(dbPassed *sqlx.DB) *ReservationServiceServer {
	err := ensureReservationTableExists(dbPassed)

	if err != nil {
		log.Fatalf("Failed to create the table Reservation: %v", err)
	}

	return &ReservationServiceServer{db: dbPassed}
}

func (s *ReservationServiceServer) CreateReservation(ctx context.Context, req *reservations.CreateReservationRequest) (*reservations.CreateReservationResponse, error) {
	reservation := req.Reservation

	if reservation == nil {
		return nil, fmt.Errorf("invalid reservation: reservation is nil")
	}

	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to RoomService: %v", err)
	}
	defer conn.Close()

	roomClient := roomspb.NewRoomServiceClient(conn)

	roomReq := &roomspb.GetRoomRequest{Id: reservation.RoomId}
	roomRes, err := roomClient.GetRoom(ctx, roomReq)

	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, fmt.Errorf("Room not found")
		}
		return nil, fmt.Errorf("Error while contacting RoomService: %v", err)
	}

	log.Printf("Room found: %v", roomRes)

	var count int

	query := `SELECT COUNT(*) FROM reservations WHERE roomId = $1 AND startingDate = $2`
	err = s.db.GetContext(ctx, &count, query, reservation.RoomId, reservation.StartingDate.AsTime())

	if err != nil {
		return nil, err
	}

	if count > 0 {
		return nil, fmt.Errorf("There is already a reservation for the room for that day.")
	}

	query = `INSERT INTO reservations (roomId, startingDate, endDate) VALUES ($1, $2, $3) RETURNING id`
	err = s.db.QueryRowContext(ctx, query, reservation.RoomId, reservation.StartingDate.AsTime(), reservation.EndDate.AsTime()).Scan(&reservation.Id)

	if err != nil {
		return nil, err
	}

	return &reservations.CreateReservationResponse{Reservation: reservation}, nil
}
