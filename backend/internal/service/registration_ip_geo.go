package service

import (
	"context"
	"errors"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ipgeo"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type registrationIPInfoUpdater interface {
	UpdateRegistrationIPInfo(ctx context.Context, userID int64, info RegistrationIPInfo) error
}

func (s *AuthService) refreshRegistrationIPLocationInBackground(userID int64, rawIP string) {
	if s == nil || s.userRepo == nil || userID <= 0 || strings.TrimSpace(rawIP) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		info, err := lookupRegistrationIPInfo(ctx, rawIP)
		if err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Failed to lookup registration IP location: user_id=%d ip=%s err=%v", userID, rawIP, err)
			return
		}
		if err := updateUserRegistrationIPInfoWithRepo(ctx, s.userRepo, userID, info); err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Failed to update registration IP location: user_id=%d ip=%s err=%v", userID, rawIP, err)
		}
	}()
}

func updateUserRegistrationIPInfoWithRepo(ctx context.Context, repo UserRepository, userID int64, info RegistrationIPInfo) error {
	updater, ok := repo.(registrationIPInfoUpdater)
	if !ok {
		return errors.New("registration ip updater is not configured")
	}
	return updater.UpdateRegistrationIPInfo(ctx, userID, info)
}

func lookupRegistrationIPInfo(ctx context.Context, rawIP string) (RegistrationIPInfo, error) {
	ipAddress := strings.TrimSpace(rawIP)
	if ipAddress == "" {
		return RegistrationIPInfo{}, infraerrors.BadRequest("REGISTER_IP_EMPTY", "registration IP is empty")
	}
	geo, err := ipgeo.Lookup(ctx, ipAddress)
	if err != nil {
		return RegistrationIPInfo{}, err
	}
	if geo == nil {
		return RegistrationIPInfo{IPAddress: ipAddress}, nil
	}
	return RegistrationIPInfo{
		IPAddress:   firstNonEmptyRegistrationIP(geo.IPAddress, ipAddress),
		Country:     geo.Country,
		CountryCode: geo.CountryCode,
		Region:      geo.Region,
		City:        geo.City,
		Location:    geo.Location,
	}, nil
}

func firstNonEmptyRegistrationIP(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
