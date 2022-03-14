package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jinzhu/copier"
	"github.com/shuifa/go-grpc/pb"
)

var (
	ErrAlreadyExists = errors.New("laptop id already exists")
)

type LaptopStore interface {
	Save(laptop *pb.Laptop) error
	Find(id string) (*pb.Laptop, error)

	Search(filter *pb.Filter, found func(laptop *pb.Laptop) error) error
}

type InmemoryLaptopStore struct {
	data map[string]*pb.Laptop
	mtx  *sync.RWMutex
}

func NewInmemoryLaptopStore() *InmemoryLaptopStore {
	return &InmemoryLaptopStore{
		data: make(map[string]*pb.Laptop),
		mtx:  new(sync.RWMutex),
	}
}

func (store *InmemoryLaptopStore) Save(laptop *pb.Laptop) error {
	store.mtx.Lock()
	defer store.mtx.Unlock()

	if store.data[laptop.Id] != nil {
		return ErrAlreadyExists
	}

	other, err := deepCopy(laptop)
	if err != nil {
		return fmt.Errorf("cannot copy from laptop %w", err)
	}

	store.data[other.Id] = other

	return nil
}

func (store *InmemoryLaptopStore) Find(id string) (*pb.Laptop, error) {
	store.mtx.RLock()
	defer store.mtx.RUnlock()

	if store.data[id] == nil {
		return nil, nil
	}

	return deepCopy(store.data[id])
}

func (store *InmemoryLaptopStore) Search(
	filter *pb.Filter,
	found func(laptop *pb.Laptop,
	) error) error {

	store.mtx.RLock()
	defer store.mtx.RUnlock()

	for _, laptop := range store.data {
		if isQualified(filter, laptop) {
			time.Sleep(time.Second)
			other, err := deepCopy(laptop)
			if err != nil {
				return err
			}
			err = found(other)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func isQualified(filter *pb.Filter, laptop *pb.Laptop) bool {
	if laptop.GetPriceUsd() > filter.GetMaxPriceUsd() {
		return false
	}

	if laptop.GetCpu().NumberCors < filter.GetMinCpuCores() {
		return false
	}

	if laptop.GetCpu().MinGhz < filter.GetMinCpuGhz() {
		return false
	}

	if toBit(laptop.GetRam()) < toBit(filter.GetMinRam()) {
		return false
	}
	return true
}

func toBit(memory *pb.Memory) uint64 {
	value := memory.GetValue()
	switch memory.GetUnit() {
	case pb.Memory_BIT:
		return value
	case pb.Memory_BYTE:
		return value << 3
	case pb.Memory_KILOBYTE:
		return value << 13
	case pb.Memory_MEGABYTE:
		return value << 23
	case pb.Memory_GIGABYTE:
		return value << 33
	case pb.Memory_TERABYTE:
		return value << 43
	default:
		return 0
	}
}

func deepCopy(laptop *pb.Laptop) (*pb.Laptop, error) {
	other := &pb.Laptop{}

	err := copier.Copy(other, laptop)

	return other, err
}
