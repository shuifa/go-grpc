package service

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"
)

type ImageStore interface {
	save(laptopID string, imageType string, imageData bytes.Buffer) (string, error)
}

type DiskImageStore struct {
	imageFolder string
	mutex       sync.RWMutex
	images      map[string]*ImageInfo
}

type ImageInfo struct {
	LaptopId string
	Type     string
	Path     string
}

func NewDiskImageStore(folder string) *DiskImageStore {
	return &DiskImageStore{imageFolder: folder, images: make(map[string]*ImageInfo)}
}

func (store *DiskImageStore) save(
	laptopID string,
	imageType string,
	imageData bytes.Buffer,
) (string, error) {
	imageID, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("cannot gennerate imageID: %w", err)
	}

	imagePath := fmt.Sprintf("%s/%s%s", store.imageFolder, imageID, imageType)

	err = os.WriteFile(imagePath, imageData.Bytes(), 0666)
	if err != nil {
		return "", fmt.Errorf("cannot wtite data to file: %w", err)
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	imageInfo := &ImageInfo{
		LaptopId: laptopID,
		Type:     imageType,
		Path:     imagePath,
	}

	store.images[imageID.String()] = imageInfo

	return imageID.String(), nil
}
