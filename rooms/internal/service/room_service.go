package service

import (
	"Hotelium/rooms/pkg/rooms"
	"context"
	"github.com/jmoiron/sqlx"
	"log"
)

type RoomServiceServer struct {
	rooms.UnimplementedRoomServiceServer
	db *sqlx.DB
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
	room := req.Room

	query := `INSERT INTO rooms (name, roomType, price) VALUES ($1, $2, $3) RETURNING id`
	err := s.db.QueryRowContext(ctx, query, room.Name, room.RoomType, room.Price).Scan(&room.Id)

	if err != nil {
		return nil, err
	}
	return &rooms.CreateRoomResponse{Room: room}, nil
}

func (s *RoomServiceServer) GetRoom(ctx context.Context, req *rooms.GetRoomRequest) (*rooms.GetRoomResponse, error) {

	var room rooms.Room

	query := `SELECT * FROM rooms WHERE id = $1`
	err := s.db.GetContext(ctx, &room, query, req.Id)

	if err != nil {
		return nil, err
	}

	log.Println("Room found: %v", room.GetId())

	return &rooms.GetRoomResponse{Room: &room}, nil
}

func (s *RoomServiceServer) ListRooms(ctx context.Context, req *rooms.ListRoomsRequest) (*rooms.ListRoomsResponse, error) {
	var listRooms []*rooms.Room

	query := `SELECT * FROM rooms`
	err := s.db.SelectContext(ctx, &listRooms, query)

	if err != nil {
		log.Fatalf(err.Error())
		return nil, err
	}

	return &rooms.ListRoomsResponse{Rooms: listRooms}, nil
}
