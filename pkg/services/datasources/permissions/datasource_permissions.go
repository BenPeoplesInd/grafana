package permissions

import (
	"context"
	"errors"

	"github.com/grafana/grafana/pkg/models"
)

var ErrNotImplemented = errors.New("not implemented")

type DatasourcePermissionsService interface {
	FilterDatasourcesBasedOnQueryPermissions(ctx context.Context, cmd *models.DatasourcesPermissionFilterQuery) error
}

// dummy method
func (hs *OSSDatasourcePermissionsService) FilterDatasourcesBasedOnQueryPermissions(ctx context.Context, cmd *models.DatasourcesPermissionFilterQuery) error {
	//return ErrNotImplemented
	return nil
}

type OSSDatasourcePermissionsService struct{}

func ProvideDatasourcePermissionsService() *OSSDatasourcePermissionsService {
	return &OSSDatasourcePermissionsService{}
}
