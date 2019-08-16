package api

import (
	"fmt"
	"net/http"

	echo "github.com/labstack/echo/v4"
	"gitlab.swisscloud.io/appc-cf-core/appcloud-backman-app/util"
)

func (h *Handler) ListServices(c echo.Context) error {
	serviceType := c.QueryParam("service_type")
	serviceName := c.QueryParam("service_name")

	services, err := h.Service.GetServices(serviceType, serviceName)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return c.JSON(http.StatusOK, services)
}

func (h *Handler) ListBackups(c echo.Context) error {
	serviceType := c.QueryParam("service_type")
	serviceName := c.QueryParam("service_name")

	backups, err := h.Service.GetBackups(serviceType, serviceName)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return c.JSON(http.StatusOK, backups)
}

func (h *Handler) GetBackup(c echo.Context) error {
	serviceType := c.Param("service_type")
	serviceName := c.Param("service_name")
	filename := c.Param("file")

	backup, err := h.Service.GetBackup(serviceType, serviceName, filename)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if len(backup.Files) == 0 || len(backup.Files[0].Filename) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Errorf("file not found"))
	}
	return c.JSON(http.StatusOK, backup)
}

func (h *Handler) CreateBackup(c echo.Context) error {
	serviceType := c.Param("service_type")
	serviceName := c.Param("service_name")
	filename := c.Param("file")

	if !util.IsValidServiceType(serviceType) {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("unsupported service type: %s", serviceType))
	}

	go func() {
		_ = h.Service.Backup(serviceType, serviceName, filename) // async
	}()
	return c.JSON(http.StatusAccepted, nil)
}

func (h *Handler) DownloadBackup(c echo.Context) error {
	serviceType := c.Param("service_type")
	serviceName := c.Param("service_name")
	filename := c.Param("file")

	reader, err := h.Service.ReadBackup(serviceType, serviceName, filename)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Stream(http.StatusOK, "application/gzip", reader)
}

func (h *Handler) DeleteBackup(c echo.Context) error {
	serviceType := c.Param("service_type")
	serviceName := c.Param("service_name")
	filename := c.Param("file")

	if err := h.Service.DeleteBackup(serviceType, serviceName, filename); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusNoContent, nil)
}