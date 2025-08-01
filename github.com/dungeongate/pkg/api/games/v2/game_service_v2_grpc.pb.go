// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.3
// source: api/proto/games/game_service_v2.proto

package v2

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	GameService_ListGames_FullMethodName        = "/dungeongate.games.v2.GameService/ListGames"
	GameService_GetGame_FullMethodName          = "/dungeongate.games.v2.GameService/GetGame"
	GameService_CreateGame_FullMethodName       = "/dungeongate.games.v2.GameService/CreateGame"
	GameService_UpdateGame_FullMethodName       = "/dungeongate.games.v2.GameService/UpdateGame"
	GameService_DeleteGame_FullMethodName       = "/dungeongate.games.v2.GameService/DeleteGame"
	GameService_StartGameSession_FullMethodName = "/dungeongate.games.v2.GameService/StartGameSession"
	GameService_StopGameSession_FullMethodName  = "/dungeongate.games.v2.GameService/StopGameSession"
	GameService_GetGameSession_FullMethodName   = "/dungeongate.games.v2.GameService/GetGameSession"
	GameService_ListGameSessions_FullMethodName = "/dungeongate.games.v2.GameService/ListGameSessions"
	GameService_SaveGame_FullMethodName         = "/dungeongate.games.v2.GameService/SaveGame"
	GameService_LoadGame_FullMethodName         = "/dungeongate.games.v2.GameService/LoadGame"
	GameService_DeleteSave_FullMethodName       = "/dungeongate.games.v2.GameService/DeleteSave"
	GameService_ListSaves_FullMethodName        = "/dungeongate.games.v2.GameService/ListSaves"
	GameService_StreamGameIO_FullMethodName     = "/dungeongate.games.v2.GameService/StreamGameIO"
	GameService_ResizeTerminal_FullMethodName   = "/dungeongate.games.v2.GameService/ResizeTerminal"
	GameService_AddSpectator_FullMethodName     = "/dungeongate.games.v2.GameService/AddSpectator"
	GameService_RemoveSpectator_FullMethodName  = "/dungeongate.games.v2.GameService/RemoveSpectator"
	GameService_Health_FullMethodName           = "/dungeongate.games.v2.GameService/Health"
)

// GameServiceClient is the client API for GameService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// GameService provides game management operations
type GameServiceClient interface {
	// Game management
	ListGames(ctx context.Context, in *ListGamesRequest, opts ...grpc.CallOption) (*ListGamesResponse, error)
	GetGame(ctx context.Context, in *GetGameRequest, opts ...grpc.CallOption) (*GetGameResponse, error)
	CreateGame(ctx context.Context, in *CreateGameRequest, opts ...grpc.CallOption) (*CreateGameResponse, error)
	UpdateGame(ctx context.Context, in *UpdateGameRequest, opts ...grpc.CallOption) (*UpdateGameResponse, error)
	DeleteGame(ctx context.Context, in *DeleteGameRequest, opts ...grpc.CallOption) (*DeleteGameResponse, error)
	// Session management
	StartGameSession(ctx context.Context, in *StartGameSessionRequest, opts ...grpc.CallOption) (*StartGameSessionResponse, error)
	StopGameSession(ctx context.Context, in *StopGameSessionRequest, opts ...grpc.CallOption) (*StopGameSessionResponse, error)
	GetGameSession(ctx context.Context, in *GetGameSessionRequest, opts ...grpc.CallOption) (*GetGameSessionResponse, error)
	ListGameSessions(ctx context.Context, in *ListGameSessionsRequest, opts ...grpc.CallOption) (*ListGameSessionsResponse, error)
	// Save management
	SaveGame(ctx context.Context, in *SaveGameRequest, opts ...grpc.CallOption) (*SaveGameResponse, error)
	LoadGame(ctx context.Context, in *LoadGameRequest, opts ...grpc.CallOption) (*LoadGameResponse, error)
	DeleteSave(ctx context.Context, in *DeleteSaveRequest, opts ...grpc.CallOption) (*DeleteSaveResponse, error)
	ListSaves(ctx context.Context, in *ListSavesRequest, opts ...grpc.CallOption) (*ListSavesResponse, error)
	// PTY streaming for terminal I/O
	StreamGameIO(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[GameIORequest, GameIOResponse], error)
	ResizeTerminal(ctx context.Context, in *ResizeTerminalRequest, opts ...grpc.CallOption) (*ResizeTerminalResponse, error)
	// Spectator management
	AddSpectator(ctx context.Context, in *AddSpectatorRequest, opts ...grpc.CallOption) (*AddSpectatorResponse, error)
	RemoveSpectator(ctx context.Context, in *RemoveSpectatorRequest, opts ...grpc.CallOption) (*RemoveSpectatorResponse, error)
	// Health check
	Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HealthResponse, error)
}

type gameServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewGameServiceClient(cc grpc.ClientConnInterface) GameServiceClient {
	return &gameServiceClient{cc}
}

func (c *gameServiceClient) ListGames(ctx context.Context, in *ListGamesRequest, opts ...grpc.CallOption) (*ListGamesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListGamesResponse)
	err := c.cc.Invoke(ctx, GameService_ListGames_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) GetGame(ctx context.Context, in *GetGameRequest, opts ...grpc.CallOption) (*GetGameResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetGameResponse)
	err := c.cc.Invoke(ctx, GameService_GetGame_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) CreateGame(ctx context.Context, in *CreateGameRequest, opts ...grpc.CallOption) (*CreateGameResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CreateGameResponse)
	err := c.cc.Invoke(ctx, GameService_CreateGame_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) UpdateGame(ctx context.Context, in *UpdateGameRequest, opts ...grpc.CallOption) (*UpdateGameResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateGameResponse)
	err := c.cc.Invoke(ctx, GameService_UpdateGame_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) DeleteGame(ctx context.Context, in *DeleteGameRequest, opts ...grpc.CallOption) (*DeleteGameResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteGameResponse)
	err := c.cc.Invoke(ctx, GameService_DeleteGame_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) StartGameSession(ctx context.Context, in *StartGameSessionRequest, opts ...grpc.CallOption) (*StartGameSessionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(StartGameSessionResponse)
	err := c.cc.Invoke(ctx, GameService_StartGameSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) StopGameSession(ctx context.Context, in *StopGameSessionRequest, opts ...grpc.CallOption) (*StopGameSessionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(StopGameSessionResponse)
	err := c.cc.Invoke(ctx, GameService_StopGameSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) GetGameSession(ctx context.Context, in *GetGameSessionRequest, opts ...grpc.CallOption) (*GetGameSessionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetGameSessionResponse)
	err := c.cc.Invoke(ctx, GameService_GetGameSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) ListGameSessions(ctx context.Context, in *ListGameSessionsRequest, opts ...grpc.CallOption) (*ListGameSessionsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListGameSessionsResponse)
	err := c.cc.Invoke(ctx, GameService_ListGameSessions_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) SaveGame(ctx context.Context, in *SaveGameRequest, opts ...grpc.CallOption) (*SaveGameResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SaveGameResponse)
	err := c.cc.Invoke(ctx, GameService_SaveGame_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) LoadGame(ctx context.Context, in *LoadGameRequest, opts ...grpc.CallOption) (*LoadGameResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LoadGameResponse)
	err := c.cc.Invoke(ctx, GameService_LoadGame_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) DeleteSave(ctx context.Context, in *DeleteSaveRequest, opts ...grpc.CallOption) (*DeleteSaveResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteSaveResponse)
	err := c.cc.Invoke(ctx, GameService_DeleteSave_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) ListSaves(ctx context.Context, in *ListSavesRequest, opts ...grpc.CallOption) (*ListSavesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListSavesResponse)
	err := c.cc.Invoke(ctx, GameService_ListSaves_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) StreamGameIO(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[GameIORequest, GameIOResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &GameService_ServiceDesc.Streams[0], GameService_StreamGameIO_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GameIORequest, GameIOResponse]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type GameService_StreamGameIOClient = grpc.BidiStreamingClient[GameIORequest, GameIOResponse]

func (c *gameServiceClient) ResizeTerminal(ctx context.Context, in *ResizeTerminalRequest, opts ...grpc.CallOption) (*ResizeTerminalResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ResizeTerminalResponse)
	err := c.cc.Invoke(ctx, GameService_ResizeTerminal_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) AddSpectator(ctx context.Context, in *AddSpectatorRequest, opts ...grpc.CallOption) (*AddSpectatorResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AddSpectatorResponse)
	err := c.cc.Invoke(ctx, GameService_AddSpectator_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) RemoveSpectator(ctx context.Context, in *RemoveSpectatorRequest, opts ...grpc.CallOption) (*RemoveSpectatorResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RemoveSpectatorResponse)
	err := c.cc.Invoke(ctx, GameService_RemoveSpectator_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *gameServiceClient) Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HealthResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(HealthResponse)
	err := c.cc.Invoke(ctx, GameService_Health_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GameServiceServer is the server API for GameService service.
// All implementations must embed UnimplementedGameServiceServer
// for forward compatibility.
//
// GameService provides game management operations
type GameServiceServer interface {
	// Game management
	ListGames(context.Context, *ListGamesRequest) (*ListGamesResponse, error)
	GetGame(context.Context, *GetGameRequest) (*GetGameResponse, error)
	CreateGame(context.Context, *CreateGameRequest) (*CreateGameResponse, error)
	UpdateGame(context.Context, *UpdateGameRequest) (*UpdateGameResponse, error)
	DeleteGame(context.Context, *DeleteGameRequest) (*DeleteGameResponse, error)
	// Session management
	StartGameSession(context.Context, *StartGameSessionRequest) (*StartGameSessionResponse, error)
	StopGameSession(context.Context, *StopGameSessionRequest) (*StopGameSessionResponse, error)
	GetGameSession(context.Context, *GetGameSessionRequest) (*GetGameSessionResponse, error)
	ListGameSessions(context.Context, *ListGameSessionsRequest) (*ListGameSessionsResponse, error)
	// Save management
	SaveGame(context.Context, *SaveGameRequest) (*SaveGameResponse, error)
	LoadGame(context.Context, *LoadGameRequest) (*LoadGameResponse, error)
	DeleteSave(context.Context, *DeleteSaveRequest) (*DeleteSaveResponse, error)
	ListSaves(context.Context, *ListSavesRequest) (*ListSavesResponse, error)
	// PTY streaming for terminal I/O
	StreamGameIO(grpc.BidiStreamingServer[GameIORequest, GameIOResponse]) error
	ResizeTerminal(context.Context, *ResizeTerminalRequest) (*ResizeTerminalResponse, error)
	// Spectator management
	AddSpectator(context.Context, *AddSpectatorRequest) (*AddSpectatorResponse, error)
	RemoveSpectator(context.Context, *RemoveSpectatorRequest) (*RemoveSpectatorResponse, error)
	// Health check
	Health(context.Context, *emptypb.Empty) (*HealthResponse, error)
	mustEmbedUnimplementedGameServiceServer()
}

// UnimplementedGameServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedGameServiceServer struct{}

func (UnimplementedGameServiceServer) ListGames(context.Context, *ListGamesRequest) (*ListGamesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListGames not implemented")
}
func (UnimplementedGameServiceServer) GetGame(context.Context, *GetGameRequest) (*GetGameResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetGame not implemented")
}
func (UnimplementedGameServiceServer) CreateGame(context.Context, *CreateGameRequest) (*CreateGameResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateGame not implemented")
}
func (UnimplementedGameServiceServer) UpdateGame(context.Context, *UpdateGameRequest) (*UpdateGameResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateGame not implemented")
}
func (UnimplementedGameServiceServer) DeleteGame(context.Context, *DeleteGameRequest) (*DeleteGameResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteGame not implemented")
}
func (UnimplementedGameServiceServer) StartGameSession(context.Context, *StartGameSessionRequest) (*StartGameSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StartGameSession not implemented")
}
func (UnimplementedGameServiceServer) StopGameSession(context.Context, *StopGameSessionRequest) (*StopGameSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopGameSession not implemented")
}
func (UnimplementedGameServiceServer) GetGameSession(context.Context, *GetGameSessionRequest) (*GetGameSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetGameSession not implemented")
}
func (UnimplementedGameServiceServer) ListGameSessions(context.Context, *ListGameSessionsRequest) (*ListGameSessionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListGameSessions not implemented")
}
func (UnimplementedGameServiceServer) SaveGame(context.Context, *SaveGameRequest) (*SaveGameResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SaveGame not implemented")
}
func (UnimplementedGameServiceServer) LoadGame(context.Context, *LoadGameRequest) (*LoadGameResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LoadGame not implemented")
}
func (UnimplementedGameServiceServer) DeleteSave(context.Context, *DeleteSaveRequest) (*DeleteSaveResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteSave not implemented")
}
func (UnimplementedGameServiceServer) ListSaves(context.Context, *ListSavesRequest) (*ListSavesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSaves not implemented")
}
func (UnimplementedGameServiceServer) StreamGameIO(grpc.BidiStreamingServer[GameIORequest, GameIOResponse]) error {
	return status.Errorf(codes.Unimplemented, "method StreamGameIO not implemented")
}
func (UnimplementedGameServiceServer) ResizeTerminal(context.Context, *ResizeTerminalRequest) (*ResizeTerminalResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ResizeTerminal not implemented")
}
func (UnimplementedGameServiceServer) AddSpectator(context.Context, *AddSpectatorRequest) (*AddSpectatorResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddSpectator not implemented")
}
func (UnimplementedGameServiceServer) RemoveSpectator(context.Context, *RemoveSpectatorRequest) (*RemoveSpectatorResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveSpectator not implemented")
}
func (UnimplementedGameServiceServer) Health(context.Context, *emptypb.Empty) (*HealthResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Health not implemented")
}
func (UnimplementedGameServiceServer) mustEmbedUnimplementedGameServiceServer() {}
func (UnimplementedGameServiceServer) testEmbeddedByValue()                     {}

// UnsafeGameServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to GameServiceServer will
// result in compilation errors.
type UnsafeGameServiceServer interface {
	mustEmbedUnimplementedGameServiceServer()
}

func RegisterGameServiceServer(s grpc.ServiceRegistrar, srv GameServiceServer) {
	// If the following call pancis, it indicates UnimplementedGameServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&GameService_ServiceDesc, srv)
}

func _GameService_ListGames_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListGamesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).ListGames(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_ListGames_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).ListGames(ctx, req.(*ListGamesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_GetGame_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetGameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).GetGame(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_GetGame_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).GetGame(ctx, req.(*GetGameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_CreateGame_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateGameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).CreateGame(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_CreateGame_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).CreateGame(ctx, req.(*CreateGameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_UpdateGame_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateGameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).UpdateGame(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_UpdateGame_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).UpdateGame(ctx, req.(*UpdateGameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_DeleteGame_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteGameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).DeleteGame(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_DeleteGame_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).DeleteGame(ctx, req.(*DeleteGameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_StartGameSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StartGameSessionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).StartGameSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_StartGameSession_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).StartGameSession(ctx, req.(*StartGameSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_StopGameSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StopGameSessionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).StopGameSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_StopGameSession_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).StopGameSession(ctx, req.(*StopGameSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_GetGameSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetGameSessionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).GetGameSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_GetGameSession_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).GetGameSession(ctx, req.(*GetGameSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_ListGameSessions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListGameSessionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).ListGameSessions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_ListGameSessions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).ListGameSessions(ctx, req.(*ListGameSessionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_SaveGame_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SaveGameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).SaveGame(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_SaveGame_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).SaveGame(ctx, req.(*SaveGameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_LoadGame_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LoadGameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).LoadGame(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_LoadGame_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).LoadGame(ctx, req.(*LoadGameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_DeleteSave_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteSaveRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).DeleteSave(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_DeleteSave_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).DeleteSave(ctx, req.(*DeleteSaveRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_ListSaves_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSavesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).ListSaves(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_ListSaves_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).ListSaves(ctx, req.(*ListSavesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_StreamGameIO_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(GameServiceServer).StreamGameIO(&grpc.GenericServerStream[GameIORequest, GameIOResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type GameService_StreamGameIOServer = grpc.BidiStreamingServer[GameIORequest, GameIOResponse]

func _GameService_ResizeTerminal_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ResizeTerminalRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).ResizeTerminal(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_ResizeTerminal_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).ResizeTerminal(ctx, req.(*ResizeTerminalRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_AddSpectator_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddSpectatorRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).AddSpectator(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_AddSpectator_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).AddSpectator(ctx, req.(*AddSpectatorRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_RemoveSpectator_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveSpectatorRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).RemoveSpectator(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_RemoveSpectator_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).RemoveSpectator(ctx, req.(*RemoveSpectatorRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _GameService_Health_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GameServiceServer).Health(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GameService_Health_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GameServiceServer).Health(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// GameService_ServiceDesc is the grpc.ServiceDesc for GameService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var GameService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "dungeongate.games.v2.GameService",
	HandlerType: (*GameServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListGames",
			Handler:    _GameService_ListGames_Handler,
		},
		{
			MethodName: "GetGame",
			Handler:    _GameService_GetGame_Handler,
		},
		{
			MethodName: "CreateGame",
			Handler:    _GameService_CreateGame_Handler,
		},
		{
			MethodName: "UpdateGame",
			Handler:    _GameService_UpdateGame_Handler,
		},
		{
			MethodName: "DeleteGame",
			Handler:    _GameService_DeleteGame_Handler,
		},
		{
			MethodName: "StartGameSession",
			Handler:    _GameService_StartGameSession_Handler,
		},
		{
			MethodName: "StopGameSession",
			Handler:    _GameService_StopGameSession_Handler,
		},
		{
			MethodName: "GetGameSession",
			Handler:    _GameService_GetGameSession_Handler,
		},
		{
			MethodName: "ListGameSessions",
			Handler:    _GameService_ListGameSessions_Handler,
		},
		{
			MethodName: "SaveGame",
			Handler:    _GameService_SaveGame_Handler,
		},
		{
			MethodName: "LoadGame",
			Handler:    _GameService_LoadGame_Handler,
		},
		{
			MethodName: "DeleteSave",
			Handler:    _GameService_DeleteSave_Handler,
		},
		{
			MethodName: "ListSaves",
			Handler:    _GameService_ListSaves_Handler,
		},
		{
			MethodName: "ResizeTerminal",
			Handler:    _GameService_ResizeTerminal_Handler,
		},
		{
			MethodName: "AddSpectator",
			Handler:    _GameService_AddSpectator_Handler,
		},
		{
			MethodName: "RemoveSpectator",
			Handler:    _GameService_RemoveSpectator_Handler,
		},
		{
			MethodName: "Health",
			Handler:    _GameService_Health_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamGameIO",
			Handler:       _GameService_StreamGameIO_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "api/proto/games/game_service_v2.proto",
}
