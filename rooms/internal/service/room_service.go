package service

import (
	"Hotelium/rooms/pkg/rooms"
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"sync"
)

type RoomServiceServer struct {
	rooms.UnimplementedRoomServiceServer
	db      *sqlx.DB
	dbMutex sync.Mutex
}

func ensureRoomTableExists(db *sqlx.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS rooms (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    roomType VARCHAR NOT NULL,
    price NUMERIC NOT NULL
);`

	_, err := db.Exec(query)

	return err
}

func NewRoomServiceServer(dbPassed *sqlx.DB) *RoomServiceServer {
	err := ensureRoomTableExists(dbPassed)
	if err != nil {
		log.Fatalf("Failed to create the table Rooms: %v", err)
	}
	return &RoomServiceServer{db: dbPassed}
}

func (s *RoomServiceServer) CreateRoom(ctx context.Context, req *rooms.CreateRoomRequest) (*rooms.CreateRoomResponse, error) {

	s.dbMutex.Lock()
	room := req.Room

	query := `INSERT INTO rooms (name, roomType, price) VALUES ($1, $2, $3) RETURNING id`
	err := s.db.QueryRowContext(ctx, query, room.Name, room.RoomType, room.Price).Scan(&room.Id)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create room: %v")
	}

	s.dbMutex.Unlock()
	return &rooms.CreateRoomResponse{Room: room}, nil
}

func (s *RoomServiceServer) GetRoom(ctx context.Context, req *rooms.GetRoomRequest) (*rooms.GetRoomResponse, error) {

	var room rooms.Room

	query := `SELECT * FROM rooms WHERE id = $1`
	err := s.db.GetContext(ctx, &room, query, req.Id)

	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Room not found")
	}

	log.Println("Room found: %v", room.GetId())

	return &rooms.GetRoomResponse{Room: &room}, nil
}

func (s *RoomServiceServer) ListRooms(ctx context.Context, req *rooms.ListRoomsRequest) (*rooms.ListRoomsResponse, error) {
	var listRooms []*rooms.Room

	query := `SELECT * FROM rooms`
	err := s.db.SelectContext(ctx, &listRooms, query)

	if err != nil {
		return nil, err
	}

	return &rooms.ListRoomsResponse{Rooms: listRooms}, nil
}

func (s *RoomServiceServer) SearchRoomByName(ctx context.Context, req *rooms.SearchRoomByNameRequest) (*rooms.SearchRoomByNameResponse, error) {
	var listRooms []*rooms.Room

	roomNameFormatted := fmt.Sprintf("%%%s%%", req.RoomName)

	query := `SELECT * FROM rooms WHERE name LIKE $1`

	err := s.db.SelectContext(ctx, &listRooms, query, roomNameFormatted)

	if err != nil {
		return nil, err
	}

	return &rooms.SearchRoomByNameResponse{Room: listRooms}, nil
}

func (s *RoomServiceServer) UpdateRoom(ctx context.Context, req *rooms.UpdateRoomRequest) (*rooms.UpdateRoomResponse, error) {
	room := req.Room

	query := `UPDATE rooms SET name = $1, roomType = $2, price = $3 WHERE id = $4`

	_, err := s.db.ExecContext(ctx, query, &room.Name, &room.RoomType, &room.Price, &room.Id)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to update room: %v")
	}

	return &rooms.UpdateRoomResponse{Room: room}, nil
}
