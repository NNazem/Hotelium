package service

import (
	"Hotelium/reservations/pkg/reservations"
	roomspb "Hotelium/reservations/pkg/rooms"
	"context"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"time"
)

type ReservationServiceServer struct {
	reservations.UnimplementedReservationServiceServer
	db         *sqlx.DB
	roomClient roomspb.RoomServiceClient
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

	//addresses := []string{"localhost:50051", "localhost:50052"}

	if err != nil {
		log.Fatalf("Failed to create the table Reservation: %v", err)
	}

	conn, err := grpc.Dial(
		"localhost:50051,localhost:50052", // Use a comma-separated list of addresses
		grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // Enable round-robin load balancing
	)
	if err != nil {
		log.Printf("Failed to connect to server: %v", err)
	}

	roomClient := roomspb.NewRoomServiceClient(conn)

	return &ReservationServiceServer{db: dbPassed, roomClient: roomClient}
}

func (s *ReservationServiceServer) CreateReservation(ctx context.Context, req *reservations.CreateReservationRequest) (*reservations.CreateReservationResponse, error) {
	reservation := req.Reservation

	if reservation == nil {
		log.Printf("Reservation can't be nil")
		return nil, errors.New("reservation cannot be nil")
	}

	if reservation.StartingDate == "" {
		log.Printf("Reservation starting date can't be empty")
		return nil, errors.New("startingDate cannot be blank")
	}

	if reservation.EndDate == "" {
		log.Printf("Reservation ending date can't be empty")
		return nil, errors.New("endDate cannot be blank")
	}

	log.Printf("Startint the creation of a reservation for room: %v", req.Reservation.RoomId)

	startingTimeParsed, endDateParsed, err := validateReservationDates(reservation.StartingDate, reservation.EndDate)

	if err != nil {
		return nil, err
	}
	ctx, _ = context.WithTimeout(ctx, 5*time.Second)

	err = s.isValidRoomId(ctx, reservation)

	if err != nil {
		return nil, err
	}

	var exists bool

	query := `SELECT EXISTS(SELECT 1 FROM reservations WHERE roomId = $1 AND startingDate = $2)`
	err = s.db.GetContext(ctx, &exists, query, reservation.RoomId, startingTimeParsed)

	if err != nil {
		return nil, err
	}

	if exists {
		log.Printf("Reservation with roomId %v already exists", req.Reservation.RoomId)
		return nil, fmt.Errorf("There is already a reservation for the room for that day.")
	}

	query = `INSERT INTO reservations (roomId, startingDate, endDate) VALUES ($1, $2, $3) RETURNING id`
	err = s.db.QueryRowContext(ctx, query, reservation.RoomId, startingTimeParsed, endDateParsed).Scan(&reservation.Id)

	if err != nil {
		return nil, err
	}

	log.Printf("Reservation created for room: %v", req.Reservation.RoomId)
	sendEmail("Reservation confirmed", "Reservation confirmed")

	return &reservations.CreateReservationResponse{Reservation: reservation}, nil
}

func (s *ReservationServiceServer) CancelReservation(ctx context.Context, req *reservations.CancelReservationRequest) (*reservations.CancelReservationResponse, error) {
	id := req.Id

	if id < 0 {
		log.Printf("The provided reservatio id is not valid.")
		return nil, errors.New("Reservation cannot be cancelled, the provided reservatio id is not valid.")
	}

	query := `DELETE from reservations WHERE id = $1`
	result, err := s.db.ExecContext(ctx, query, id)

	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		log.Printf("Reservation with id %v not found", id)
		return nil, errors.New("Reservation cannot be cancelled, the provided reservation id is not valid.")
	}

	sendEmail("Reservation cancelled", "Reservation cancelled")

	return &reservations.CancelReservationResponse{Message: "Reservation cancelled successfully"}, nil
}

func (s *ReservationServiceServer) RoomAvailability(ctx context.Context, req *reservations.RoomAvailabilityRequest) (*reservations.RoomAvailabilityResponse, error) {
	var roomsResults []roomspb.Room
	var availableRooms []*roomspb.Room

	if req.GetStartingDate() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "startingDate is required")
	}

	query := `SELECT * FROM rooms WHERE id NOT IN (
    SELECT r.id FROM rooms as r 
    JOIN reservations as re ON r.id = re.roomid 
    WHERE re.startingdate >= $1)`

	err := s.db.SelectContext(ctx, &roomsResults, query, req.GetStartingDate())

	if err != nil {
		return nil, err
	}

	for _, room := range roomsResults {
		availableRooms = append(availableRooms, &roomspb.Room{
			Id:       room.Id,
			Name:     room.Name,
			RoomType: room.RoomType,
			Price:    room.Price,
		})
	}

	return &reservations.RoomAvailabilityResponse{Rooms: availableRooms}, nil
}

func validateReservationDates(startingDate string, endDate string) (time.Time, time.Time, error) {
	startingTimeParsed, err := time.Parse(time.RFC3339, startingDate)

	if err != nil {
		log.Printf("Failed to parse the starting date: %v", err)
		return time.Time{}, time.Time{}, fmt.Errorf("invalid starting date: %v", startingDate)
	}

	endDateParsed, err := time.Parse(time.RFC3339, endDate)

	if err != nil {
		log.Printf("Failed to parse the ending date: %v", err)
		return time.Time{}, time.Time{}, fmt.Errorf("invalid ending date: %v", endDate)
	}

	if startingTimeParsed.Before(time.Now()) {
		log.Printf("The starting date cannot be in the past")
		return time.Time{}, time.Time{}, fmt.Errorf("Invalid reservation: starting date can't be before today %v", startingDate)
	}

	if endDateParsed.Before(startingTimeParsed) {
		log.Printf("The ending date cannot be before the starting date")
		return time.Time{}, time.Time{}, fmt.Errorf("Invalid reservation: ending date can't be before starting date %v", endDate)
	}

	return startingTimeParsed, endDateParsed, nil
}

func sendEmail(subject, body string) error {
	from := mail.NewEmail("Hotelium", "projectxmk2@gmail.com")
	to := mail.NewEmail("Navid Nazem", "nazem.navid@gmail.com")
	plainTextContent := body
	htmlContent := fmt.Sprintf("<strong>%s</strong>", body)
	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)

	client := sendgrid.NewSendClient("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")
	response, err := client.Send(message)
	if err != nil {
		log.Printf("Failed to send email: %v", err)
		return err
	}

	log.Printf("Email sent successfully: %v", response)
	return nil
}

func (s *ReservationServiceServer) isValidRoomId(ctx context.Context, reservation *reservations.Reservation) error {

	roomReq := &roomspb.GetRoomRequest{Id: reservation.RoomId}
	_, err := s.roomClient.GetRoom(ctx, roomReq)

	if err != nil {
		log.Printf("Failed to get room: %v", err)
		return fmt.Errorf("Room with id: %v not found", reservation.RoomId)
	}

	return err
}
