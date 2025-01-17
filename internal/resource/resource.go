package resource

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/gofrs/uuid"
	"github.com/instill-ai/model-backend/config"
	"github.com/instill-ai/model-backend/pkg/constant"
	"github.com/instill-ai/model-backend/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	mgmtPB "github.com/instill-ai/protogen-go/base/mgmt/v1alpha"
)

// ExtractFromMetadata extracts context metadata given a key
func ExtractFromMetadata(ctx context.Context, key string) ([]string, bool) {
	data, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return []string{}, false
	}
	return data[strings.ToLower(key)], true
}

// GetRequestSingleHeader get a request header, the header has to be single-value HTTP header
func GetRequestSingleHeader(ctx context.Context, header string) string {
	metaHeader := metadata.ValueFromIncomingContext(ctx, strings.ToLower(header))
	if len(metaHeader) != 1 {
		return ""
	}
	return metaHeader[0]
}

// GetOwnerCustom returns the resource owner from a request
func GetOwnerCustom(req *http.Request, client mgmtPB.MgmtPrivateServiceClient, redisClient *redis.Client) (*mgmtPB.User, error) {
	logger, _ := logger.GetZapLogger(req.Context())

	// Verify if "authorization" is in the header
	authorization := req.Header.Get(constant.HeaderAuthorization)
	// Verify if "jwt-sub" is in the header
	headerOwnerUId := req.Header.Get(constant.HeaderOwnerUIDKey)
	apiToken := strings.Replace(authorization, "Bearer ", "", 1)
	if apiToken != "" {
		ownerPermalink, err := redisClient.Get(context.Background(), fmt.Sprintf(constant.AccessTokenKeyFormat, apiToken)).Result()
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "Unauthorized")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := client.LookUpUserAdmin(ctx, &mgmtPB.LookUpUserAdminRequest{Permalink: ownerPermalink})
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "Unauthorized")
		}

		return resp.User, nil

	} else if headerOwnerUId != "" {
		_, err := uuid.FromString(headerOwnerUId)
		if err != nil {
			logger.Error(err.Error())
			return nil, status.Errorf(codes.NotFound, "Not found")
		}

		ownerPermalink := "users/" + headerOwnerUId
		resp, err := client.LookUpUserAdmin(req.Context(), &mgmtPB.LookUpUserAdminRequest{Permalink: ownerPermalink})
		if err != nil {
			logger.Error(err.Error())
			return nil, status.Errorf(codes.NotFound, "Not found")
		}
		return resp.GetUser(), nil

	} else {
		// Verify "owner-id" in the header if there is no "jwt-sub"
		headerOwnerId := req.Header.Get(constant.HeaderOwnerIDKey)
		if headerOwnerId == "" {
			logger.Error("'owner-id' not found in the header")
			return nil, status.Errorf(codes.Unauthenticated, "Unauthorized")
		}

		ownerName := "users/" + headerOwnerId
		resp, err := client.GetUserAdmin(req.Context(), &mgmtPB.GetUserAdminRequest{Name: ownerName})
		if err != nil {
			logger.Error(err.Error())
			return nil, status.Errorf(codes.NotFound, "Not found")
		}
		return resp.GetUser(), nil
	}
}

// GetOwner returns the resource owner
func GetOwner(ctx context.Context, client mgmtPB.MgmtPrivateServiceClient, redisClient *redis.Client) (*mgmtPB.User, error) {
	logger, _ := logger.GetZapLogger(ctx)

	// Verify if "authorization" is in the header
	authorization := GetRequestSingleHeader(ctx, constant.HeaderAuthorization)
	// Verify if "jwt-sub" is in the header
	headerOwnerUId := GetRequestSingleHeader(ctx, constant.HeaderOwnerUIDKey)
	apiToken := strings.Replace(authorization, "Bearer ", "", 1)
	if apiToken != "" {
		ownerPermalink, err := redisClient.Get(context.Background(), fmt.Sprintf(constant.AccessTokenKeyFormat, apiToken)).Result()
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "Unauthorized")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := client.LookUpUserAdmin(ctx, &mgmtPB.LookUpUserAdminRequest{Permalink: ownerPermalink})
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "Unauthorized")
		}

		return resp.User, nil

	} else if headerOwnerUId != "" {
		_, err := uuid.FromString(headerOwnerUId)
		if err != nil {
			logger.Error(fmt.Sprintf("[mgmt-backend] %s", err.Error()))
			return nil, status.Errorf(codes.NotFound, "Not found")
		}
		ownerPermalink := "users/" + headerOwnerUId

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := client.LookUpUserAdmin(ctx, &mgmtPB.LookUpUserAdminRequest{Permalink: ownerPermalink})
		if err != nil {
			logger.Error(fmt.Sprintf("[mgmt-backend] %s", err.Error()))
			return nil, status.Errorf(codes.NotFound, "Not found")
		}

		return resp.User, nil
	}

	// Verify "owner-id" in the header if there is no "jwt-sub"
	headerOwnerId := GetRequestSingleHeader(ctx, constant.HeaderOwnerIDKey)
	if headerOwnerId == "" {
		return nil, status.Errorf(codes.Unauthenticated, "Unauthorized")
	}

	// Get the permalink from management backend from resource name
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.GetUserAdmin(ctx, &mgmtPB.GetUserAdminRequest{Name: "users/" + headerOwnerId})
	if err != nil {
		logger.Error(fmt.Sprintf("[mgmt-backend] %s", err.Error()))
		return nil, status.Errorf(codes.NotFound, "Not found")
	}
	return resp.User, nil
}

// GetOwnerWithAPIToken returns the resource owner that strictly required api token
func GetOwnerWithAPIToken(ctx context.Context, client mgmtPB.MgmtPrivateServiceClient, redisClient *redis.Client) (*mgmtPB.User, error) {
	// Verify if "authorization" is in the header
	authorization := GetRequestSingleHeader(ctx, constant.HeaderAuthorization)
	// temporary solution to restrict cloud version from calling APIs without header
	// need further concrete design of authentication
	if strings.HasPrefix(config.Config.Server.Edition, "cloud") && authorization == "" {
		return nil, status.Errorf(codes.Unauthenticated, "Unauthorized")
	} else {
		return GetOwner(ctx, client, redisClient)
	}
}

func GetID(name string) (string, error) {
	id := strings.TrimPrefix(name, "models/")
	if !strings.HasPrefix(name, "models/") || id == "" {
		return "", status.Error(codes.InvalidArgument, "Error when extract models resource id")
	}
	return id, nil
}

func GetModelID(name string) (string, error) {
	if match, _ := regexp.MatchString(`^models/.+$`, name); !match {
		return "", status.Error(codes.InvalidArgument, "Error when extract models resource id")
	}
	subs := strings.Split(name, "/")
	return subs[1], nil
}

func GetDefinitionID(name string) (string, error) {
	id := strings.TrimPrefix(name, "model-definitions/")
	if !strings.HasPrefix(name, "model-definitions/") || id == "" {
		return "", status.Error(codes.InvalidArgument, "Error when extract model-definitions resource id")
	}
	return id, nil
}

// GetPermalinkUID returns the resource UID given a resource permalink
func GetPermalinkUID(permalink string) (string, error) {
	uid := permalink[strings.LastIndex(permalink, "/")+1:]
	if uid == "" {
		return "", status.Errorf(codes.InvalidArgument, "Error when extract resource id from resource permalink `%s`", permalink)
	}
	return uid, nil
}

func GetOperationID(name string) (string, error) {
	id := strings.TrimPrefix(name, "operations/")
	if !strings.HasPrefix(name, "operations/") || id == "" {
		return "", status.Error(codes.InvalidArgument, "Error when extract operations resource id")
	}
	return id, nil
}
