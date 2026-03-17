package repositories

import (
	"app-sistem-akuntansi/models"
	"fmt"

	"gorm.io/gorm"
)

type ProjectRepository interface {
	FindAll() ([]models.Project, error)
	FindByID(id uint) (*models.Project, error)
	Create(project *models.Project) error
	Update(project *models.Project) error
	Delete(id uint) error
}

type projectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &projectRepository{db: db}
}

func (r *projectRepository) FindAll() ([]models.Project, error) {
	var projects []models.Project
	if err := r.db.Preload("Manager").Find(&projects).Error; err != nil {
		return nil, err
	}

	if err := r.fillSalesMetrics(projects); err != nil {
		return nil, err
	}

	return projects, nil
}

func (r *projectRepository) FindByID(id uint) (*models.Project, error) {
	var project models.Project
	err := r.db.Preload("Manager").First(&project, id).Error
	if err != nil {
		return nil, err
	}

	projects := []models.Project{project}
	if err := r.fillSalesMetrics(projects); err != nil {
		return nil, err
	}
	project = projects[0]

	return &project, nil
}

func (r *projectRepository) Create(project *models.Project) error {
	return r.db.Create(project).Error
}

func (r *projectRepository) Update(project *models.Project) error {
	return r.db.Save(project).Error
}

func (r *projectRepository) Delete(id uint) error {
	return r.db.Delete(&models.Project{}, id).Error
}

type projectSalesMetricRow struct {
	ProjectID           uint
	RecognizedRevenue   float64
	SalesPipelineAmount float64
}

func (r *projectRepository) fillSalesMetrics(projects []models.Project) error {
	if len(projects) == 0 {
		return nil
	}

	projectIDs := make([]uint, 0, len(projects))
	for _, project := range projects {
		projectIDs = append(projectIDs, project.ID)
	}

	var rows []projectSalesMetricRow
	if err := r.db.Model(&models.Sale{}).
		Select(`
			project_id,
			COALESCE(SUM(CASE WHEN status IN ('INVOICED', 'OVERDUE', 'PAID') THEN total_amount ELSE 0 END), 0) AS recognized_revenue,
			COALESCE(SUM(CASE WHEN status IN ('DRAFT', 'CONFIRMED', 'PENDING') THEN total_amount ELSE 0 END), 0) AS sales_pipeline_amount
		`).
		Where("project_id IN ? AND deleted_at IS NULL", projectIDs).
		Group("project_id").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("failed to calculate project sales metrics: %w", err)
	}

	metricsByProject := make(map[uint]projectSalesMetricRow, len(rows))
	for _, row := range rows {
		metricsByProject[row.ProjectID] = row
	}

	for i := range projects {
		metric := metricsByProject[projects[i].ID]
		projects[i].TotalSalesRevenue = metric.RecognizedRevenue
		projects[i].SalesPipelineAmount = metric.SalesPipelineAmount
	}

	return nil
}
