package main

import (
	"context"
	"fmt"
	"go-todo/proto/protoc"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type server struct {
	protoc.UnimplementedTodoManagerServer
	DB *gorm.DB
}

type Todo struct {
	gorm.Model
	Name string
}

// func (UnimplementedTodoManagerServer) CreateTodoItem(context.Context, *CreateTodo) (*Todo, error) {
// 	return nil, status.Errorf(codes.Unimplemented, "method CreateTodoItem not implemented")
// }
// func (UnimplementedTodoManagerServer) GetTodoList(context.Context, *emptypb.Empty) (*Todos, error) {
// 	return nil, status.Errorf(codes.Unimplemented, "method GetTodoList not implemented")
// }
// func (UnimplementedTodoManagerServer) GetTodoItemByID(context.Context, *TodoId) (*Todo, error) {
// 	return nil, status.Errorf(codes.Unimplemented, "method GetTodoItemByID not implemented")
// }
// func (UnimplementedTodoManagerServer) DeleteTodo(context.Context, *TodoId) (*ConfirmMessage, error) {
// 	return nil, status.Errorf(codes.Unimplemented, "method DeleteTodo not implemented")
// }
// func (UnimplementedTodoManagerServer) UpdateTodo(context.Context, *TodoId) (*Todo, error) {
// 	return nil, status.Errorf(codes.Unimplemented, "method UpdateTodo not implemented")
// }
// func (UnimplementedTodoManagerServer) mustEmbedUnimplementedTodoManagerServer() {}

func (s *server) CreateTodoItem(_ context.Context, req *protoc.CreateTodo) (*protoc.Todo, error) {

	if req.Name != "" {
		todo := &Todo{
			Name: req.Name,
		}
		s.DB.Create(todo)

		return &protoc.Todo{
			Name: todo.Name,
			Id:   int32(todo.ID),
		}, nil
	}
	return nil, status.Errorf(codes.InvalidArgument, "Todo name is null")
}

func (s *server) GetTodoList(context.Context, *emptypb.Empty) (*protoc.Todos, error) {
	var todo []*protoc.Todo

	s.DB.Find(&todo)

	return &protoc.Todos{
		Todos: todo,
	}, nil
}

func (s *server) DeleteTodo(_ context.Context, req *protoc.TodoId) (*protoc.ConfirmMessage, error) {
	fmt.Println(req.Id)
	var todo *protoc.Todo
	result := s.DB.Delete(&todo, req.Id)

	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return &protoc.ConfirmMessage{
			Message: "Todo not exist",
		}, nil
	}

	return &protoc.ConfirmMessage{
		Message: "Delete successfully",
	}, nil
}

func (s *server) UpdateTodo(_ context.Context, req *protoc.Todo) (*protoc.Todo, error) {
	var todo *protoc.Todo
	result := s.DB.Find(&todo, req.Id)

	if result.Error != nil {
		return nil, result.Error
	}

	if req.Name != "" {
		todo.Name = req.Name
	}

	s.DB.Save(&todo)
	return todo, nil
}

func (s *server) GetTodoItemByID(_ context.Context, req *protoc.TodoId) (*protoc.Todo, error) {
	var todo *protoc.Todo
	result := s.DB.Find(&todo, req.Id)

	if result.Error != nil {
		return nil, result.Error
	}

	return todo, nil
}

func NewServer(db *gorm.DB) *server {
	return &server{
		DB: db,
	}
}

func main() {
	// TODO need create db and change db url
	db, err := gorm.Open(mysql.Open("root:@tcp(127.0.0.1:3306)/todo_list?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&Todo{})

	// Create a listener on TCP port
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	// Create a gRPC server object
	s := grpc.NewServer()

	// Attach the Greeter service to the server
	protoc.RegisterTodoManagerServer(s, NewServer(db))
	// Serve gRPC server
	log.Println("Serving gRPC on 0.0.0.0:8080")
	go func() {
		log.Fatalln(s.Serve(lis))
	}()

	// Create a client connection to the gRPC server we just started
	// This is where the gRPC-Gateway proxies the requests
	conn, err := grpc.DialContext(
		context.Background(),
		"0.0.0.0:8080",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalln("Failed to dial server:", err)
	}

	gwmux := runtime.NewServeMux()
	// Register Greeter
	err = protoc.RegisterTodoManagerHandler(context.Background(), gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register gateway:", err)
	}

	gwServer := &http.Server{
		Addr:    ":8090",
		Handler: gwmux,
	}

	log.Println("Serving gRPC-Gateway on http://0.0.0.0:8090")
	log.Fatalln(gwServer.ListenAndServe())

}
