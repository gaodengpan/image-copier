package entities

import (
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
)

type Image struct {
	id          *value_objects.ImageID
	credentials *value_objects.Credentials
	endpoint    *value_objects.RegistryEndpoint
	status      ImageStatus
	createdAt   time.Time
	updatedAt   time.Time
}

type ImageStatus int

const (
	StatusPending ImageStatus = iota
	StatusLocalExists
	StatusRegistryExists
	StatusSyncing
	StatusCompleted
	StatusFailed
)

func NewImage(imageID string) (*Image, error) {
	id, err := value_objects.NewImageID(imageID)
	if err != nil {
		return nil, err
	}

	return &Image{
		id:        id,
		status:    StatusPending,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}, nil
}

func (i *Image) ID() *value_objects.ImageID { return i.id }
func (i *Image) Status() ImageStatus        { return i.status }
func (i *Image) CreatedAt() time.Time       { return i.createdAt }
func (i *Image) UpdatedAt() time.Time       { return i.updatedAt }

func (i *Image) SetCredentials(username, password string) error {
	creds, err := value_objects.NewCredentials(username, password)
	if err != nil {
		return err
	}
	i.credentials = creds
	i.updatedAt = time.Now()
	return nil
}

func (i *Image) Credentials() *value_objects.Credentials {
	return i.credentials
}

func (i *Image) SetEndpoint(host, namespace, arch, os string) {
	i.endpoint = value_objects.NewRegistryEndpoint(host, namespace, arch, os)
	i.updatedAt = time.Now()
}

func (i *Image) Endpoint() *value_objects.RegistryEndpoint {
	return i.endpoint
}

func (i *Image) BuildDestImageID() string {
	if i.endpoint == nil {
		return i.id.String()
	}
	return i.endpoint.BuildDestImageID(i.id.String())
}

func (i *Image) MarkAsLocalExists() {
	i.status = StatusLocalExists
	i.updatedAt = time.Now()
}

func (i *Image) MarkAsRegistryExists() {
	i.status = StatusRegistryExists
	i.updatedAt = time.Now()
}

func (i *Image) MarkAsSyncing() {
	i.status = StatusSyncing
	i.updatedAt = time.Now()
}

func (i *Image) MarkAsCompleted() {
	i.status = StatusCompleted
	i.updatedAt = time.Now()
}

func (i *Image) MarkAsFailed() {
	i.status = StatusFailed
	i.updatedAt = time.Now()
}

func (i *Image) IsReadyToSync() bool {
	return i.status == StatusPending && i.credentials != nil
}

func (i *Image) StatusString() string {
	switch i.status {
	case StatusPending:
		return "pending"
	case StatusLocalExists:
		return "local_exists"
	case StatusRegistryExists:
		return "registry_exists"
	case StatusSyncing:
		return "syncing"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}
