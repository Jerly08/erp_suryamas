package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
)

type ProjectService interface {
	GetAllProjects() ([]models.Project, error)
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

func (s *projectService) GetProjectByID(id uint) (*models.Project, error) {
	return s.repo.FindByID(id)
}

func (s *projectService) CreateProject(req models.ProjectCreateRequest) (*models.Project, error) {
	project := &models.Project{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Status:      models.ProjectStatusActive,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		TotalBudget: req.TotalBudget,
		ManagerID:   req.ManagerID,
	}

	if err := s.repo.Create(project); err != nil {
		return nil, err
	}
	return project, nil
}

func (s *projectService) UpdateProject(id uint, req models.ProjectUpdateRequest) (*models.Project, error) {
	project, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.Status != nil {
		project.Status = *req.Status
	}
	if req.StartDate != nil {
		project.StartDate = req.StartDate
	}
	if req.EndDate != nil {
		project.EndDate = req.EndDate
	}
	if req.TotalBudget != nil {
		project.TotalBudget = *req.TotalBudget
	}
	if req.ManagerID != nil {
		project.ManagerID = req.ManagerID
	}

	if err := s.repo.Update(project); err != nil {
		return nil, err
	}

	return project, nil
}

func (s *projectService) DeleteProject(id uint) error {
	return s.repo.Delete(id)
}
