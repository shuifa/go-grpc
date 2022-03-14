package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/shuifa/go-grpc/pb"
	"github.com/shuifa/go-grpc/service"
	"google.golang.org/grpc"
)

func main() {

	port := flag.Int("port", 0, "the server port")
	flag.Parse()

	log.Printf("start server port on %d", *port)

	laptopStore := service.NewInmemoryLaptopStore()
	imageStore := service.NewDiskImageStore("img")
	ratingStore := service.NewInmemoryRatingStore()
	laptopServer := service.NewLaptopServer(laptopStore, imageStore, ratingStore)

	grpcServer := grpc.NewServer()
	pb.RegisterLaptopServiceServer(grpcServer, laptopServer)

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal(err)
	}
}
