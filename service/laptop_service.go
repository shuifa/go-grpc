package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/google/uuid"
	"github.com/shuifa/go-grpc/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LaptopServer struct {
	laptopStore LaptopStore
	imageStore  ImageStore
	ratingStore RatingStore
}

func (server *LaptopServer) RateLaptop(stream pb.LaptopService_RateLaptopServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			log.Print("接收完毕")
			break
		}
		if err != nil {
			return logErr(status.Errorf(codes.Unknown, "recv err:", err))
		}

		find, err := server.laptopStore.Find(req.GetLaptopId())
		if err != nil {
			return logErr(status.Errorf(codes.Internal, "find laptop err :%v", err))
		}

		if find == nil {
			return logErr(status.Errorf(codes.NotFound, "laptopID: %s not found", req.GetLaptopId()))
		}

		rating, err := server.ratingStore.Add(req.GetLaptopId(), req.GetScore())
		if err != nil {
			return logErr(status.Errorf(codes.Internal, "add err:%v", err))
		}

		log.Printf("评分增加成功。id：%s，评分次数：%d，总分：%f", req.LaptopId, rating.Count, rating.Sum)

		rsp := &pb.RateLaptopResponse{
			LaptopId:     req.LaptopId,
			RatingCount:  rating.Count,
			AverageScore: rating.Sum / float64(rating.Count),
		}

		err = stream.Send(rsp)
		if err != nil {
			return logErr(status.Errorf(codes.Unknown, "send err:%v", err))
		}
	}
	return nil
}

func (server *LaptopServer) UploadImage(stream pb.LaptopService_UploadImageServer) error {
	req, err := stream.Recv()
	if err != nil {
		return logErr(status.Errorf(codes.Unknown, "接收请求错误: %w", err))
	}

	laptopID := req.GetInfo().LaptopId
	imageType := req.GetInfo().ImageType

	log.Printf("rcv reuest with laptopID:%s, iamgeType:%s", laptopID, imageType)

	laptop, err := server.laptopStore.Find(laptopID)
	if err != nil {
		return logErr(status.Errorf(codes.Internal, "查找laptop错误：%w", err))
	}

	if laptop == nil {
		return logErr(status.Errorf(codes.NotFound, "laptop 不存在，id：%s", laptopID))
	}

	imageData := bytes.Buffer{}
	imageSize := 0

	for {
		log.Print("等待接收更多数据")
		data, err := stream.Recv()
		if err == io.EOF {
			log.Print("no more data")
			break
		}
		if err != nil {
			return logErr(status.Errorf(codes.Unknown, "获取数据错误：%w", err))
		}

		imageSize += len(data.GetChunkData())
		if imageSize > 2<<20 {
			return logErr(status.Errorf(codes.OutOfRange, "大小超过限制：当前：%d，允许：%d", imageSize, 2<<20))
		}
		_, err = imageData.Write(data.GetChunkData())
		if err != nil {
			return logErr(status.Errorf(codes.Internal, "写入缓冲区失败：%w", err))
		}
	}

	imageId, err := server.imageStore.save(laptopID, imageType, imageData)
	if err != nil {
		return logErr(status.Errorf(codes.Internal, "写入数据库失败，错误：%w", err))
	}

	rsp := &pb.UploadImageResponse{
		Id:   imageId,
		Size: uint32(imageSize),
	}

	err = stream.SendAndClose(rsp)
	if err != nil {
		return logErr(status.Errorf(codes.Unknown, "send and close err: %w", err))
	}

	log.Printf("imageId:%s save suceess", imageId)

	return nil
}

func logErr(err error) error {
	if err != nil {
		log.Print(err)
	}
	return err
}

func NewLaptopServer(laptopStore LaptopStore, imageStore ImageStore, ratingStore RatingStore) *LaptopServer {
	return &LaptopServer{laptopStore: laptopStore, imageStore: imageStore, ratingStore: ratingStore}
}

func (server *LaptopServer) SearchLaptop(request *pb.SearchLaptopRequest, stream pb.LaptopService_SearchLaptopServer) error {
	filter := request.GetFilter()

	log.Printf("rcv filter: %v", filter)

	err := server.laptopStore.Search(filter, func(laptop *pb.Laptop) error {
		rsp := &pb.SearchLaptopResponse{Laptop: laptop}
		log.Printf("laptop found with id: %s", laptop.Id)
		err := stream.Send(rsp)
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

func (server *LaptopServer) CreateLaptop(ctx context.Context, request *pb.CreateLaptopRequest) (*pb.CreateLaptopResponse, error) {

	laptop := request.GetLaptop()

	fmt.Printf("receive a create request, laptop id %s", laptop.Id)

	if len(laptop.Id) > 0 {
		_, err := uuid.Parse(laptop.Id)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "laptop ID is not a invalid uuid: %v", err)
		}
	} else {
		ID, err := uuid.NewRandom()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot generate a uuid: %v", err)
		}
		laptop.Id = ID.String()
	}

	if ctx.Err() == context.Canceled {
		return nil, status.Errorf(codes.Canceled, "cancel call")
	}

	err := server.laptopStore.Save(laptop)
	if err != nil {
		code := codes.Internal
		if errors.Is(err, ErrAlreadyExists) {
			code = codes.AlreadyExists
		}
		return nil, status.Errorf(code, "cannot save to store %w", err)
	}

	log.Printf("save laptop to store with id: %s", laptop.Id)

	rsp := &pb.CreateLaptopResponse{
		Id: laptop.Id,
	}

	return rsp, nil
}
