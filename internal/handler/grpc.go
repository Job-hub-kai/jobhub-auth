package handler

import (
	"context"
	"errors"

	authpb "github.com/Alksndr9/jobhub-proto/gen/go/auth"
	"github.com/job-hub-kai/jobhub-auth/internal/domain"
	"github.com/job-hub-kai/jobhub-auth/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	authpb.UnimplementedAuthServiceServer
	svc *service.AuthService
	log *zap.Logger
}

func NewGRPCHandler(svc *service.AuthService, log *zap.Logger) *GRPCHandler {
	return &GRPCHandler{svc: svc, log: log}
}

func (h *GRPCHandler) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	userID, err := h.svc.Register(ctx, domain.RegisterInput{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
		Name:     req.GetName(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authpb.RegisterResponse{UserId: userID}, nil
}

func (h *GRPCHandler) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	pair, err := h.svc.Login(ctx, domain.LoginInput{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authpb.LoginResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

func (h *GRPCHandler) Logout(ctx context.Context, req *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	if err := h.svc.Logout(ctx, req.GetAccessToken()); err != nil {
		return nil, toGRPCError(err)
	}
	return &authpb.LogoutResponse{}, nil
}

func (h *GRPCHandler) RefreshToken(ctx context.Context, req *authpb.RefreshTokenRequest) (*authpb.RefreshTokenResponse, error) {
	pair, err := h.svc.RefreshToken(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authpb.RefreshTokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

func (h *GRPCHandler) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	userID, email, err := h.svc.ValidateToken(ctx, req.GetAccessToken())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authpb.ValidateTokenResponse{
		UserId: userID,
		Email:  email,
	}, nil
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUserAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidPassword):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrTokenExpired):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrTokenBlacklisted):
		return status.Error(codes.Unauthenticated, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
