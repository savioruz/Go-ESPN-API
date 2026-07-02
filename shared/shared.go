// Package shared provides common utility functions used across the application.
//
//nolint:revive
package shared

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-espn-api/config"
	"go-espn-api/shared/cache"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dto"
	"go-espn-api/shared/timezone"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// ConvertStringToBool converts a string value to a boolean pointer.
func ConvertStringToBool(value string) *bool {
	if value == "" {
		return nil
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		log.Error().Err(err).Msg("failed to convert string to bool")

		return nil
	}

	return &boolValue
}

// CalculateTotalPage calculates the total number of pages based on total items and limit per page.
func CalculateTotalPage(total, limit int) (res int) {
	if total == 0 || limit <= 0 {
		res = 0
	} else {
		res = int(math.Ceil(float64(total) / float64(limit)))
	}

	return res
}

// TransformFields converts the fields of a struct into a map of updated fields.
func TransformFields(data interface{}, user string) map[string]any {
	val := reflect.ValueOf(data)
	typ := reflect.TypeOf(data)

	updatedFields := make(map[string]any)

	for index := range val.NumField() {
		field := val.Field(index)
		if field.IsZero() {
			continue
		}

		fieldName := typ.Field(index).Tag.Get("db")
		if fieldName == "" {
			continue
		}

		updatedFields[fieldName] = field.Interface()
	}

	updatedFields[constant.FieldModifiedAt] = timezone.NowUTC()
	updatedFields[constant.FieldModifiedBy] = user

	return updatedFields
}

// SingleFilter creates a FilterGroup to filter by a single field.
func SingleFilter(value, field, table string) dto.FilterGroup {
	return dto.FilterGroup{
		Filters: []any{
			dto.Filter{
				Field:    field,
				Value:    value,
				Operator: dto.FilterOperatorEq,
				Table:    table,
			},
		},
	}
}

// BuildCacheKey builds a cache key with optional postfix parts.
func BuildCacheKey(key string, postfix ...string) string {
	cfg := config.Get()
	parent := cfg.App.Name

	if len(postfix) > 0 {
		suffix := strings.Join(postfix, ":")

		return fmt.Sprintf("%s:cache:%s:%s", parent, key, suffix)
	}

	return fmt.Sprintf("%s:cache:%s", parent, key)
}

// BuildCacheKeyWithQuery builds a cache key including query parameters and filters.
func BuildCacheKeyWithQuery(key string, queryParams dto.QueryParams, filter dto.FilterGroup) string {
	cfg := config.Get()
	parent := cfg.App.Name

	queryHash := generateQueryHash(queryParams, filter)

	return fmt.Sprintf("%s:cache:%s:%s", parent, key, queryHash)
}

func generateQueryHash(queryParams dto.QueryParams, filter dto.FilterGroup) string {
	queryData := struct {
		QueryParams dto.QueryParams `json:"query_params"`
		Filter      dto.FilterGroup `json:"filter"`
	}{
		QueryParams: queryParams,
		Filter:      filter,
	}

	jsonData, err := json.Marshal(queryData)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal query data for cache key")

		return fmt.Sprintf("page_%d_limit_%d_sortBy_%s_sortDir_%s",
			queryParams.Page, queryParams.Limit, queryParams.SortBy, queryParams.SortDir)
	}

	hash := md5.Sum(jsonData)

	return hex.EncodeToString(hash[:])
}

// InvalidateCaches clears all cache entries matching the given key pattern.
func InvalidateCaches(ctx context.Context, cache cache.RedisCache, key string) {
	if err := cache.Clear(ctx, BuildCacheKey(key, constant.Asterix)); err != nil {
		log.Error().Err(err).Msgf("failed to clear cache for key: %s", key)
	}
}

// GenerateUniqueFilename generates a unique filename with timestamp and original extension
func GenerateUniqueFilename(originalFilename string) string {
	timestamp := timezone.NowUTC().Unix()
	parts := strings.Split(originalFilename, ".")
	extension := ""

	if len(parts) > 1 {
		extension = "." + parts[len(parts)-1]
	}

	hash := md5.Sum([]byte(fmt.Sprintf("%s_%d", originalFilename, timestamp)))
	hashStr := hex.EncodeToString(hash[:])

	return fmt.Sprintf("%d_%s%s", timestamp, hashStr[:8], extension)
}
