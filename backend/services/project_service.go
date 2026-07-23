package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/utils"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ProjectService interface {
	GetAllProjects() ([]models.Project, error)
	GetProjects(filter models.ProjectFilter) ([]models.Project, int64, error)
	GetProjectByID(id uint) (*models.Project, error)
	CreateProject(req models.ProjectCreateRequest) (*models.Project, error)
	UpdateProject(id uint, req models.ProjectUpdateRequest) (*models.Project, error)
	DeleteProject(id uint) error
}

type projectService struct {
	repo repositories.ProjectRepository
}

func NewProjectService(repo repositories.ProjectRepository) ProjectService {
	return &projectService{repo: repo}
}

func (s *projectService) GetAllProjects() ([]models.Project, error) {
	return s.repo.FindAll()
}

func (s *projectService) GetProjects(filter models.ProjectFilter) ([]models.Project, int64, error) {
	return s.repo.FindWithFilter(filter)
}

func (s *projectService) GetProjectByID(id uint) (*models.Project, error) {
	project, err := s.repo.FindByID(id)
	if err != nil {
		return nil, normalizeProjectError(err, "Project")
	}
	return project, nil
}

func (s *projectService) CreateProject(req models.ProjectCreateRequest) (*models.Project, error) {
	if err := validateProjectDateRange(req.StartDate, req.EndDate); err != nil {
		return nil, err
	}

	project := &models.Project{
		Code:        strings.TrimSpace(req.Code),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Status:      models.ProjectStatusActive,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		TotalBudget: req.TotalBudget,
		ManagerID:   req.ManagerID,
	}

	if project.Code == "" {
		return nil, utils.NewValidationError("Project code is required", map[string]string{"code": "required"})
	}
	if project.Name == "" {
		return nil, utils.NewValidationError("Project name is required", map[string]string{"name": "required"})
	}

	if err := s.repo.Create(project); err != nil {
		return nil, normalizeProjectError(err, "Project")
	}
	return project, nil
}

func (s *projectService) UpdateProject(id uint, req models.ProjectUpdateRequest) (*models.Project, error) {
	project, err := s.repo.FindByID(id)
	if err != nil {
		return nil, normalizeProjectError(err, "Project")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, utils.NewValidationError("Project name is required", map[string]string{"name": "required"})
		}
		project.Name = name
	}
	if req.Description != nil {
		project.Description = strings.TrimSpace(*req.Description)
	}
	if req.Status != nil {
		if !isValidProjectStatus(*req.Status) {
			return nil, utils.NewValidationError("Invalid project status", map[string]string{"status": "invalid"})
		}
		project.Status = *req.Status
	}
	if req.ClearStartDate {
		project.StartDate = nil
	} else if req.StartDate != nil {
		project.StartDate = req.StartDate
	}
	if req.ClearEndDate {
		project.EndDate = nil
	} else if req.EndDate != nil {
		project.EndDate = req.EndDate
	}
	if req.TotalBudget != nil {
		project.TotalBudget = *req.TotalBudget
	}
	if req.ManagerID != nil {
		project.ManagerID = req.ManagerID
	}

	if err := validateProjectDateRange(project.StartDate, project.EndDate); err != nil {
		return nil, err
	}

	if err := s.repo.Update(project); err != nil {
		return nil, normalizeProjectError(err, "Project")
	}

	return project, nil
}

func (s *projectService) DeleteProject(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		return normalizeProjectError(err, "Project")
	}

	linkSummary, err := s.repo.GetLinkSummary(id)
	if err != nil {
		return normalizeProjectError(err, "Project")
	}
	if linkSummary.ProjectBOMs > 0 || linkSummary.Sales > 0 || linkSummary.Purchases > 0 || linkSummary.PurchaseRequests > 0 {
		return utils.NewConflictError("Project cannot be deleted because it already has linked BOM, sales, purchases, or purchase requests")
	}

	deleted, err := s.repo.Delete(id)
	if err != nil {
		return normalizeProjectError(err, "Project")
	}
	if !deleted {
		return utils.NewNotFoundError("Project")
	}
	return nil
}

func validateProjectDateRange(startDate *time.Time, endDate *time.Time) error {
	if startDate != nil && endDate != nil && endDate.Before(*startDate) {
		return utils.NewValidationError("End date cannot be earlier than start date", map[string]string{
			"start_date": "must be before or equal to end_date",
			"end_date":   "must be after or equal to start_date",
		})
	}
	return nil
}

func isValidProjectStatus(status string) bool {
	switch status {
	case models.ProjectStatusActive, models.ProjectStatusCompleted, models.ProjectStatusOnHold, models.ProjectStatusCancelled:
		return true
	default:
		return false
	}
}

func normalizeProjectError(err error, resource string) error {
	if err == nil {
		return nil
	}
	if appErr := utils.GetAppError(err); appErr != nil {
		return appErr
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return utils.NewNotFoundError(resource)
	}

	errText := strings.ToLower(err.Error())
	if strings.Contains(errText, "duplicate key") && strings.Contains(errText, "uni_projects_code") {
		return utils.NewConflictError("Project code already exists")
	}
	if strings.Contains(errText, "duplicate key") && strings.Contains(errText, "projects_code_key") {
		return utils.NewConflictError("Project code already exists")
	}

	return utils.NewInternalError(fmt.Sprintf("Failed to process %s", strings.ToLower(resource)), err)
}
