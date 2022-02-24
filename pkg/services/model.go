package services

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/status"
	"github.com/instill-ai/model-backend/configs"
	"github.com/instill-ai/model-backend/internal/triton"
	"github.com/instill-ai/model-backend/pkg/models"
	"github.com/instill-ai/model-backend/pkg/repository"
	"github.com/instill-ai/protogen-go/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createModelResponse(modelInDB models.Model, versions []models.Version, tritonModels []models.TModel) *models.ModelResponse {
	var vers []models.VersionResponse
	for i := 0; i < len(versions); i++ {
		vers = append(vers, models.VersionResponse{
			Version:     versions[i].Version,
			ModelId:     versions[i].ModelId,
			Description: versions[i].Description,
			CreatedAt:   versions[i].CreatedAt,
			UpdatedAt:   versions[i].UpdatedAt,
			Status:      versions[i].Status,
		})
	}
	return &models.ModelResponse{
		Name:     modelInDB.Name,
		FullName: modelInDB.FullName,
		Id:       modelInDB.Id,
		CVTask:   model.CVTask(modelInDB.CVTask).String(),
		Versions: vers,
	}
}

func createModelInfo(modelInDB models.Model, versions []models.Version, tritonModels []models.TModel) *model.ModelInfo {
	var vers []*model.ModelVersion
	for i := 0; i < len(versions); i++ {
		ver := versions[i]
		var st = model.ModelStatus_OFFLINE
		if ver.Status == model.ModelStatus_ONLINE.String() {
			st = model.ModelStatus_ONLINE
		} else if ver.Status == model.ModelStatus_ERROR.String() {
			st = model.ModelStatus_ERROR
		}

		vers = append(vers, &model.ModelVersion{
			Version:     ver.Version,
			ModelId:     ver.ModelId,
			Description: ver.Description,
			CreatedAt:   timestamppb.New(ver.CreatedAt),
			UpdatedAt:   timestamppb.New(ver.UpdatedAt),
			Status:      st,
		})
	}
	return &model.ModelInfo{
		Name:      modelInDB.Name,
		Full_Name: modelInDB.FullName,
		Id:        modelInDB.Id,
		CvTask:    model.CVTask(modelInDB.CVTask),
		Versions:  vers,
	}
}

type ModelService interface {
	CreateModel(model *models.Model) (*models.Model, error)
	GetModelByName(namespace string, modelName string) (models.Model, error)
	GetModelMetaData(namespace string, modelName string) (*model.ModelInfo, error)
	CreateVersion(version models.Version) (models.Version, error)
	UpdateModelVersion(modelId int32, modelVersion int32, versionInfo models.Version) error
	UpdateModelMetaData(namespace string, updatedModel models.Model) (*model.ModelInfo, error)
	GetModelVersion(modelId int32, version int32) (models.Version, error)
	GetModelVersions(modelId int32) ([]models.Version, error)
	PredictModelByUpload(namespace string, modelName string, version int32, imgsBytes [][]byte, cvTask model.CVTask) (interface{}, error)
	CreateModelByUpload(namespace string, createdModel *models.Model) (*model.ModelInfo, error)
	HandleCreateModelByUpload(namespace string, createdModel *models.Model) (*models.ModelResponse, error)
	ListModels(namespace string) ([]*model.ModelInfo, error)
	UpdateModel(namespace string, updatedInfo *model.UpdateModelRequest) (*model.ModelInfo, error)
	DeleteModel(namespace string, modelName string) error
	DeleteModelVersion(namespace string, modelName string, version int32) error
	GetModelVersionLatest(modelId int32) (models.Version, error)
}

type modelService struct {
	modelRepository repository.ModelRepository
}

func NewModelService(r repository.ModelRepository) ModelService {
	return &modelService{
		modelRepository: r,
	}
}

func (s *modelService) CreateModel(model *models.Model) (*models.Model, error) {
	// Validate the naming rule of model
	if match, _ := regexp.MatchString("^[A-Za-z0-9][a-zA-Z0-9_.-]*$", model.Name); !match {
		return &models.Model{}, status.Error(codes.FailedPrecondition, "The name of model is invalid")
	}

	if existingModel, _ := s.GetModelByName(model.Namespace, model.Name); existingModel.Name != "" {
		return &models.Model{}, status.Errorf(codes.FailedPrecondition, "The name %s is existing in your namespace", model.Name)
	}

	if err := s.modelRepository.CreateModel(*model); err != nil {
		return &models.Model{}, err
	}

	if createdModel, err := s.GetModelByName(model.Namespace, model.Name); err != nil {
		return &models.Model{}, err
	} else {
		return &createdModel, nil
	}
}

func (s *modelService) GetModelByName(namespace string, modelName string) (models.Model, error) {
	return s.modelRepository.GetModelByName(namespace, modelName)
}

func (s *modelService) GetModelVersionLatest(modelId int32) (models.Version, error) {
	return s.modelRepository.GetModelVersionLatest(modelId)
}

func (s *modelService) CreateVersion(version models.Version) (models.Version, error) {
	if err := s.modelRepository.CreateVersion(version); err != nil {
		return models.Version{}, err
	}

	if createdVersion, err := s.modelRepository.GetModelVersion(version.ModelId, version.Version); err != nil {
		return models.Version{}, err
	} else {
		return createdVersion, nil
	}
}

func (s *modelService) UpdateModelVersion(modelId int32, modelVersion int32, versionInfo models.Version) error {
	return s.modelRepository.UpdateModelVersion(modelId, modelVersion, versionInfo)
}

func (s *modelService) GetModelVersion(modelId int32, version int32) (models.Version, error) {
	return s.modelRepository.GetModelVersion(modelId, version)
}

func (s *modelService) GetModelVersions(modelId int32) ([]models.Version, error) {
	return s.modelRepository.GetModelVersions(modelId)
}

func (s *modelService) GetTModels(modelId int32) ([]models.TModel, error) {
	return s.modelRepository.GetTModels(modelId)
}

func (s *modelService) PredictModelByUpload(namespace string, modelName string, version int32, imgsBytes [][]byte, cvTask model.CVTask) (interface{}, error) {
	// Triton model name is change into
	modelInDB, err := s.GetModelByName(namespace, modelName)
	if err != nil {
		return nil, fmt.Errorf("Model not found")
	}

	ensembleModel, err := s.modelRepository.GetTEnsembleModel(modelInDB.Id, version)
	if err != nil {
		return nil, fmt.Errorf("Triton model not found")
	}

	ensembleModelName := ensembleModel.Name
	ensembleModelVersion := ensembleModel.Version
	modelMetadataResponse := triton.ModelMetadataRequest(triton.TritonClient, ensembleModelName, fmt.Sprint(ensembleModelVersion))
	if modelMetadataResponse == nil {
		return nil, fmt.Errorf("Model is offline")
	}
	modelConfigResponse := triton.ModelConfigRequest(triton.TritonClient, ensembleModelName, fmt.Sprint(ensembleModelVersion))
	if modelMetadataResponse == nil {
		return nil, err
	}
	// /* We use a simple model that takes 2 input tensors of 16 integers
	// each and returns 2 output tensors of 16 integers each. One
	// output tensor is the element-wise sum of the inputs and one
	// output is the element-wise difference. */
	inferResponse, err := triton.ModelInferRequest(triton.TritonClient, cvTask, imgsBytes, ensembleModelName, fmt.Sprint(ensembleModelVersion), modelMetadataResponse, modelConfigResponse)
	if err != nil {
		return nil, err
	}
	// /* We expect there to be 2 results (each with batch-size 1). Walk
	// over all 16 result elements and print the sum and difference
	// calculated by the model. */
	postprocessResponse, err := triton.PostProcess(inferResponse, modelMetadataResponse, cvTask)
	if err != nil {
		return nil, err
	}
	switch cvTask {
	case model.CVTask_CLASSIFICATION:
		clsResponses := postprocessResponse.([]string)
		var contents []*model.ClassificationOutput
		for _, clsRes := range clsResponses {
			clsResSplit := strings.Split(clsRes, ":")
			if len(clsResSplit) != 3 {
				return nil, fmt.Errorf("Unable to decode inference output")
			}
			score, err := strconv.ParseFloat(clsResSplit[0], 32)
			if err != nil {
				return nil, fmt.Errorf("Unable to decode inference output")
			}
			clsOutput := model.ClassificationOutput{
				Category: clsResSplit[2],
				Score:    float32(score),
			}
			contents = append(contents, &clsOutput)
		}
		clsOutputs := model.ClassificationOutputs{
			Contents: contents,
		}
		return &clsOutputs, nil

	case model.CVTask_DETECTION:
		detResponses := postprocessResponse.(triton.DetectionOutput)
		batchedOutputDataBboxes := detResponses.Boxes
		batchedOutputDataLabels := detResponses.Labels
		var detOutputs model.DetectionOutputs
		for i := range batchedOutputDataBboxes {
			var contents []*model.BoundingBoxPrediction
			for j := range batchedOutputDataBboxes[i] {
				box := batchedOutputDataBboxes[i][j]
				label := batchedOutputDataLabels[i][j]

				// Non-meaningful bboxes were added with coords [-1, -1, -1, -1, -1] and label "0" for Triton to be able to batch Tensors
				if label != "0" {
					pred := &model.BoundingBoxPrediction{
						Category: label,
						Score:    box[4],
						// Convert x1y1x2y2 to xywh where xy is top-left corner
						Box: &model.Box{
							Left:   box[0],
							Top:    box[1],
							Width:  box[2] - box[0],
							Height: box[3] - box[1],
						},
					}
					contents = append(contents, pred)
				}
			}
			detOutput := &model.DetectionOutput{
				Contents: contents,
			}
			detOutputs.Contents = append(detOutputs.Contents, detOutput)
		}
		return &detOutputs, nil
	default:
		return postprocessResponse, nil
	}
}

func createModel(s *modelService, namespace string, uploadedModel *models.Model) (models.Model, []models.Version, []models.TModel, error) {
	modelInDB, err := s.GetModelByName(namespace, uploadedModel.Name)
	if err != nil {
		createdModel, err := s.CreateModel(uploadedModel)
		if err != nil {
			return models.Model{}, []models.Version{}, []models.TModel{}, fmt.Errorf("Could not create model in DB")
		}
		modelInDB = *createdModel
	}

	latestVersion, err := s.modelRepository.GetModelVersionLatest(modelInDB.Id)
	if err == nil {
		uploadedModel.Versions[0].Version = latestVersion.Version + 1
	} else {
		uploadedModel.Versions[0].Version = 1
	}
	uploadedModel.Versions[0].ModelId = modelInDB.Id
	versionInDB, err := s.CreateVersion(uploadedModel.Versions[0])
	if err != nil {
		return models.Model{}, []models.Version{}, []models.TModel{}, fmt.Errorf("Could not create model version in DB")
	}
	for i := 0; i < len(uploadedModel.TritonModels); i++ {
		tmodel := uploadedModel.TritonModels[i]
		tmodel.ModelId = modelInDB.Id
		tmodel.ModelVersion = versionInDB.Version
		err = s.modelRepository.CreateTModel(tmodel)
		if err != nil {
			return models.Model{}, []models.Version{}, []models.TModel{}, fmt.Errorf("Could not create triton model in DB")
		}
	}
	versions, err := s.GetModelVersions(modelInDB.Id)
	if err != nil {
		return models.Model{}, []models.Version{}, []models.TModel{}, fmt.Errorf("Could not get model versions in DB")
	}

	return modelInDB, versions, uploadedModel.TritonModels, nil
}

func (s *modelService) CreateModelByUpload(namespace string, uploadedModel *models.Model) (*model.ModelInfo, error) {
	modelInDB, versions, tritonModels, err := createModel(s, namespace, uploadedModel)
	return createModelInfo(modelInDB, versions, tritonModels), err
}

func (s *modelService) HandleCreateModelByUpload(namespace string, uploadedModel *models.Model) (*models.ModelResponse, error) {
	modelInDB, versions, tritonModels, err := createModel(s, namespace, uploadedModel)
	return createModelResponse(modelInDB, versions, tritonModels), err
}

func (s *modelService) ListModels(namespace string) ([]*model.ModelInfo, error) {
	if !triton.IsTritonServerReady() {
		return []*model.ModelInfo{}, fmt.Errorf("Triton Server not ready yet")
	}

	models, err := s.modelRepository.ListModels(models.ListModelQuery{Namespace: namespace})
	if err != nil {
		return []*model.ModelInfo{}, err
	}

	var resModels []*model.ModelInfo
	for i := 0; i < len(models); i++ {
		md := models[i]
		versions, err := s.GetModelVersions(md.Id)
		if err != nil {
			return []*model.ModelInfo{}, err
		}
		tritonModels, err := s.GetTModels(md.Id)
		if err != nil {
			return []*model.ModelInfo{}, err

		}
		resModels = append(resModels, createModelInfo(md, versions, tritonModels))
	}

	return resModels, nil
}

func (s *modelService) UpdateModelMetaData(namespace string, updatedModel models.Model) (*model.ModelInfo, error) {
	md, err := s.GetModelByName(namespace, updatedModel.Name)
	if err != nil {
		return &model.ModelInfo{}, err
	}

	err = s.modelRepository.UpdateModelMetaData(md.Id, updatedModel)
	if err != nil {
		return &model.ModelInfo{}, err
	}

	modelInfo, err := s.GetModelMetaData(namespace, md.Name)
	if err != nil {
		return &model.ModelInfo{}, err
	}

	return modelInfo, nil
}

func (s *modelService) UpdateModel(namespace string, in *model.UpdateModelRequest) (*model.ModelInfo, error) {
	modelInDB, err := s.GetModelByName(namespace, in.Name)
	if err != nil {
		return &model.ModelInfo{}, err
	}

	if in.UpdateMask != nil && len(in.UpdateMask.Paths) > 0 {
		for _, field := range in.UpdateMask.Paths {
			switch field {
			case "status":
				ensembleModel, err := s.modelRepository.GetTEnsembleModel(modelInDB.Id, in.Version)
				if err != nil {
					return &model.ModelInfo{}, err
				}
				switch in.Model.Status {
				case model.ModelStatus_ONLINE:
					_, err = triton.LoadModelRequest(triton.TritonClient, ensembleModel.Name)
					if err != nil {
						err = s.UpdateModelVersion(modelInDB.Id, ensembleModel.ModelVersion, models.Version{
							UpdatedAt: time.Now(),
							Status:    model.ModelStatus_ERROR.String(),
						})
						if err != nil {
							return &model.ModelInfo{}, err
						}
						return &model.ModelInfo{}, err
					} else {
						err := s.UpdateModelVersion(modelInDB.Id, ensembleModel.ModelVersion, models.Version{
							UpdatedAt: time.Now(),
							Status:    model.ModelStatus_ONLINE.String(),
						})
						if err != nil {
							return &model.ModelInfo{}, err
						}
					}
				case model.ModelStatus_OFFLINE:
					_, err = triton.UnloadModelRequest(triton.TritonClient, ensembleModel.Name)
					if err != nil {
						err = s.UpdateModelVersion(modelInDB.Id, ensembleModel.ModelVersion, models.Version{
							UpdatedAt: time.Now(),
							Status:    model.ModelStatus_ERROR.String(),
						})
						if err != nil {
							return &model.ModelInfo{}, err
						}
						return &model.ModelInfo{}, err
					} else {
						err = s.UpdateModelVersion(modelInDB.Id, ensembleModel.ModelVersion, models.Version{
							UpdatedAt: time.Now(),
							Status:    model.ModelStatus_OFFLINE.String(),
						})
						if err != nil {
							return &model.ModelInfo{}, err
						}
					}
				default:
					return &model.ModelInfo{}, fmt.Errorf("Wrong status value. Status should be online or offline")
				}
			case "description":
				err = s.UpdateModelVersion(modelInDB.Id, in.Version, models.Version{
					UpdatedAt:   time.Now(),
					Description: in.Model.Description,
				})
				if err != nil {
					return &model.ModelInfo{}, err
				}
			}
		}
	}
	return s.GetModelMetaData(namespace, in.Name)
}

func (s *modelService) GetModelMetaData(namespace string, modelName string) (*model.ModelInfo, error) {
	// TODO: improve by using join
	resModelInDB, err := s.GetModelByName(namespace, modelName)
	if err != nil {
		return &model.ModelInfo{}, err
	}

	versions, err := s.GetModelVersions(resModelInDB.Id)
	if err != nil {
		return &model.ModelInfo{}, err
	}

	tritonModels, err := s.GetTModels(resModelInDB.Id)
	if err != nil {
		return &model.ModelInfo{}, err
	}

	return createModelInfo(resModelInDB, versions, tritonModels), nil
}

func (s *modelService) DeleteModel(namespace string, modelName string) error {
	modelInDB, err := s.GetModelByName(namespace, modelName)
	if err != nil {
		return err
	}

	modelVersionsInDB, err := s.GetModelVersions(modelInDB.Id)
	if err == nil {
		for i := 0; i < len(modelVersionsInDB); i++ {
			ensembleModel, err := s.modelRepository.GetTEnsembleModel(modelInDB.Id, modelVersionsInDB[i].Version)
			if err == nil {
				_, _ = triton.UnloadModelRequest(triton.TritonClient, ensembleModel.Name)
			}
		}
		tritonModels, err := s.modelRepository.GetTModels(modelInDB.Id)
		if err == nil {
			for i := 0; i < len(tritonModels); i++ {
				modelDir := filepath.Join(configs.Config.TritonServer.ModelStore, tritonModels[i].Name)
				_ = os.RemoveAll(modelDir)
			}
		}
	}

	return s.modelRepository.DeleteModel(modelInDB.Id)
}

func (s *modelService) DeleteModelVersion(namespace string, modelName string, version int32) error {
	modelInDB, err := s.GetModelByName(namespace, modelName)
	if err != nil {
		return err
	}
	modelVersionInDB, err := s.GetModelVersion(modelInDB.Id, int32(version))
	if err != nil {
		return err
	}

	ensembleModel, err := s.modelRepository.GetTEnsembleModel(modelInDB.Id, modelVersionInDB.Version)
	if err == nil {
		_, _ = triton.UnloadModelRequest(triton.TritonClient, ensembleModel.Name)
	}

	tritonModels, err := s.modelRepository.GetTModelVersions(modelInDB.Id, modelVersionInDB.Version)
	if err == nil {
		for i := 0; i < len(tritonModels); i++ {
			modelDir := filepath.Join(configs.Config.TritonServer.ModelStore, tritonModels[i].Name)
			_ = os.RemoveAll(modelDir)
		}
	}

	modelVersionsInDB, err := s.GetModelVersions(modelInDB.Id)
	if err != nil {
		return err
	}

	if len(modelVersionsInDB) > 1 {
		return s.modelRepository.DeleteModelVersion(modelInDB.Id, modelVersionInDB.Version)
	} else {
		return s.modelRepository.DeleteModel(modelInDB.Id)
	}
}