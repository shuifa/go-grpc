package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/shuifa/go-grpc/pb"
	"github.com/shuifa/go-grpc/sample"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func testCreateLaptop(laptopClient pb.LaptopServiceClient, laptop *pb.Laptop) *pb.CreateLaptopResponse {

	arg := &pb.CreateLaptopRequest{
		Laptop: laptop,
	}

	rsp, err := laptopClient.CreateLaptop(context.Background(), arg)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.AlreadyExists {
			fmt.Printf("alredy exitst %s", laptop.Id)
		} else {
			log.Fatal(err)
		}
		return nil
	}

	return rsp
}

func testSearchLaptop(laptopClient pb.LaptopServiceClient) {

	for i := 0; i < 10; i++ {
		rsp := testCreateLaptop(laptopClient, sample.NewLaptop())
		log.Printf("create laptop by id: %s", rsp.Id)
	}

	filter := &pb.Filter{
		MaxPriceUsd: 3000,
		MinCpuCores: 4,
		MinCpuGhz:   2.5,
		MinRam: &pb.Memory{
			Value: 8,
			Unit:  pb.Memory_GIGABYTE,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	req := &pb.SearchLaptopRequest{Filter: filter}
	stream, err := laptopClient.SearchLaptop(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	for {
		laptopRsp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		log.Printf(
			"found id: %s, usd:%f, cpuCores:%d, cpuGhzs:%f, ramValue:%d",
			laptopRsp.Laptop.Id,
			laptopRsp.Laptop.PriceUsd,
			laptopRsp.Laptop.GetCpu().NumberCors,
			laptopRsp.Laptop.GetCpu().MinGhz,
			laptopRsp.Laptop.GetRam().Value,
		)
	}
}

func testUploadImage(laptopClient pb.LaptopServiceClient) {
	laptop := sample.NewLaptop()
	laptopRsp := testCreateLaptop(laptopClient, laptop)

	file, err := os.OpenFile("tmp/laptop.jpg", os.O_RDONLY, 0444)
	if err != nil {
		log.Fatalf("cannot open file: %v", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	stream, err := laptopClient.UploadImage(ctx)
	if err != nil {
		return
	}

	req := &pb.UploadImageRequest{
		Data: &pb.UploadImageRequest_Info{
			Info: &pb.ImageInfo{
				LaptopId:  laptopRsp.Id,
				ImageType: filepath.Ext("tmp/laptop.jpg"),
			},
		},
	}

	err = stream.Send(req)
	if err != nil {
		log.Fatalf("cannot sned data: %v", err)
	}

	reader := bufio.NewReader(file)
	buffer := make([]byte, 1024)

	for {
		n, err := reader.Read(buffer)
		if err == io.EOF {
			log.Print("读取完毕")
			break
		}
		if err != nil {
			log.Fatalf("读取图片数据错误：%v", err)
		}
		req = &pb.UploadImageRequest{
			Data: &pb.UploadImageRequest_ChunkData{
				ChunkData: buffer[:n],
			},
		}

		err = stream.Send(req)
		if err != nil {
			log.Fatalf("发送数据错误：%v", err)
		}
	}

	recv, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("接收最后数据错误：%v", err)
	}

	log.Printf("图上传成功，id:%s，大小：%d", recv.Id, recv.Size)
}

func testRateLaptop(laptopClient pb.LaptopServiceClient) {

	var laptopIDS []string
	for i := 0; i < 3; i++ {
		rsp := testCreateLaptop(laptopClient, sample.NewLaptop())
		laptopIDS = append(laptopIDS, rsp.Id)
	}

loop:
	{
		for {
			fmt.Print("rate laptop y/n")
			var answer string
			_, err := fmt.Scan(&answer)
			if err != nil {
				log.Fatal("scan err", err)
			}

			if answer != "y" {
				goto end
			}

			goto rate
		}
	}

rate:
	{

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		stream, err := laptopClient.RateLaptop(ctx)
		if err != nil {
			log.Fatal("rate fail err:", err)
		}

		errChan := make(chan error)

		go func() {
			for {
				recv, err := stream.Recv()
				if err == io.EOF {
					errChan <- fmt.Errorf("接收完毕啦：%w", err)
					return
				}
				if err != nil {
					errChan <- fmt.Errorf("接收数据错误：%w", err)
					return
				}
				log.Printf("id：%s，评分：%f，评价次数：%d",
					recv.GetLaptopId(),
					recv.GetAverageScore(),
					recv.GetRatingCount())
			}
		}()

		for _, id := range laptopIDS {
			req := &pb.RateLaptopRequest{
				LaptopId: id,
				Score:    sample.RandomLaptopScore(),
			}
			err = stream.Send(req)
			if err != nil {
				log.Fatal("send err:", err, stream.RecvMsg(nil))
			}
		}

		err = stream.CloseSend()
		if err != nil {
			log.Fatal("send close err:", err, stream.RecvMsg(nil))
		}

		err = <-errChan

		fmt.Print("最后收到的错误信息", err)

		goto loop
	}

end:
}

func main() {

	addr := flag.String("addr", "", "server address")
	flag.Parse()

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}

	laptopClient := pb.NewLaptopServiceClient(conn)
	testRateLaptop(laptopClient)
}
