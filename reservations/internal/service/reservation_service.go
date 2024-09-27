package service

import (
	"Hotelium/reservations/pkg/reservations"
	roomspb "Hotelium/reservations/pkg/rooms"
	"context"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc"
	"log"
	"time"
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

func validateReservationDates(startingDate string, endDate string) (time.Time, time.Time, error) {
	startingTimeParsed, err := time.Parse(time.RFC3339, startingDate)

	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid starting date: %v", startingDate)
	}

	endDateParsed, err := time.Parse(time.RFC3339, endDate)

	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid ending date: %v", endDate)
	}

	if startingTimeParsed.Before(time.Now()) {
		return time.Time{}, time.Time{}, fmt.Errorf("Invalid reservation: starting date can't be before today %v", startingDate)
	}

	if endDateParsed.Before(startingTimeParsed) {
		return time.Time{}, time.Time{}, fmt.Errorf("Invalid reservation: ending date can't be before starting date %v", endDate)
	}

	return startingTimeParsed, endDateParsed, nil
}

func (s *ReservationServiceServer) CreateReservation(ctx context.Context, req *reservations.CreateReservationRequest) (*reservations.CreateReservationResponse, error) {
	reservation := req.Reservation

	if reservation == nil {
		return nil, errors.New("reservation cannot be nil")
	}

	if reservation.StartingDate == "" {
		return nil, errors.New("startingDate cannot be blank")
	}

	if reservation.EndDate == "" {
		return nil, errors.New("endDate cannot be blank")
	}

	startingTimeParsed, endDateParsed, err := validateReservationDates(reservation.StartingDate, reservation.EndDate)

	if err != nil {
		return nil, err
	}

	err = isValidRoomId(ctx, reservation)

	if err != nil {
		return nil, err
	}

	var count int

	query := `SELECT COUNT(*) FROM reservations WHERE roomId = $1 AND startingDate = $2`
	err = s.db.GetContext(ctx, &count, query, reservation.RoomId, startingTimeParsed)

	if err != nil {
		return nil, err
	}

	if count > 0 {
		return nil, fmt.Errorf("There is already a reservation for the room for that day.")
	}

	query = `INSERT INTO reservations (roomId, startingDate, endDate) VALUES ($1, $2, $3) RETURNING id`
	err = s.db.QueryRowContext(ctx, query, reservation.RoomId, startingTimeParsed, endDateParsed).Scan(&reservation.Id)

	if err != nil {
		return nil, err
	}

	return &reservations.CreateReservationResponse{Reservation: reservation}, nil
}

func isValidRoomId(ctx context.Context, reservation *reservations.Reservation) error {
	conn, err := grpc.Dial("localhost:50053", grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("Failed to connect to RoomService: %v", err)
	}
	defer conn.Close()

	roomClient := roomspb.NewRoomServiceClient(conn)

	roomReq := &roomspb.GetRoomRequest{Id: reservation.RoomId}
	_, err = roomClient.GetRoom(ctx, roomReq)

	if err != nil {
		return fmt.Errorf("Room with id: %v not found", reservation.RoomId)
	}

	return err
}
