package com.quorum.quest.api.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * LeaderElectionService provides distributed leader election capabilities
 * </pre>
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.70.0)",
    comments = "Source: v1/quorum_quest_api.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class LeaderElectionServiceGrpc {

  private LeaderElectionServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "quorum.quest.api.v1.LeaderElectionService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.quorum.quest.api.v1.TryAcquireLockRequest,
      com.quorum.quest.api.v1.TryAcquireLockResponse> getTryAcquireLockMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "TryAcquireLock",
      requestType = com.quorum.quest.api.v1.TryAcquireLockRequest.class,
      responseType = com.quorum.quest.api.v1.TryAcquireLockResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.quorum.quest.api.v1.TryAcquireLockRequest,
      com.quorum.quest.api.v1.TryAcquireLockResponse> getTryAcquireLockMethod() {
    io.grpc.MethodDescriptor<com.quorum.quest.api.v1.TryAcquireLockRequest, com.quorum.quest.api.v1.TryAcquireLockResponse> getTryAcquireLockMethod;
    if ((getTryAcquireLockMethod = LeaderElectionServiceGrpc.getTryAcquireLockMethod) == null) {
      synchronized (LeaderElectionServiceGrpc.class) {
        if ((getTryAcquireLockMethod = LeaderElectionServiceGrpc.getTryAcquireLockMethod) == null) {
          LeaderElectionServiceGrpc.getTryAcquireLockMethod = getTryAcquireLockMethod =
              io.grpc.MethodDescriptor.<com.quorum.quest.api.v1.TryAcquireLockRequest, com.quorum.quest.api.v1.TryAcquireLockResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "TryAcquireLock"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.quorum.quest.api.v1.TryAcquireLockRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.quorum.quest.api.v1.TryAcquireLockResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LeaderElectionServiceMethodDescriptorSupplier("TryAcquireLock"))
              .build();
        }
      }
    }
    return getTryAcquireLockMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.quorum.quest.api.v1.ReleaseLockRequest,
      com.quorum.quest.api.v1.ReleaseLockResponse> getReleaseLockMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ReleaseLock",
      requestType = com.quorum.quest.api.v1.ReleaseLockRequest.class,
      responseType = com.quorum.quest.api.v1.ReleaseLockResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.quorum.quest.api.v1.ReleaseLockRequest,
      com.quorum.quest.api.v1.ReleaseLockResponse> getReleaseLockMethod() {
    io.grpc.MethodDescriptor<com.quorum.quest.api.v1.ReleaseLockRequest, com.quorum.quest.api.v1.ReleaseLockResponse> getReleaseLockMethod;
    if ((getReleaseLockMethod = LeaderElectionServiceGrpc.getReleaseLockMethod) == null) {
      synchronized (LeaderElectionServiceGrpc.class) {
        if ((getReleaseLockMethod = LeaderElectionServiceGrpc.getReleaseLockMethod) == null) {
          LeaderElectionServiceGrpc.getReleaseLockMethod = getReleaseLockMethod =
              io.grpc.MethodDescriptor.<com.quorum.quest.api.v1.ReleaseLockRequest, com.quorum.quest.api.v1.ReleaseLockResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ReleaseLock"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.quorum.quest.api.v1.ReleaseLockRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.quorum.quest.api.v1.ReleaseLockResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LeaderElectionServiceMethodDescriptorSupplier("ReleaseLock"))
              .build();
        }
      }
    }
    return getReleaseLockMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.quorum.quest.api.v1.KeepAliveRequest,
      com.quorum.quest.api.v1.KeepAliveResponse> getKeepAliveMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "KeepAlive",
      requestType = com.quorum.quest.api.v1.KeepAliveRequest.class,
      responseType = com.quorum.quest.api.v1.KeepAliveResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.quorum.quest.api.v1.KeepAliveRequest,
      com.quorum.quest.api.v1.KeepAliveResponse> getKeepAliveMethod() {
    io.grpc.MethodDescriptor<com.quorum.quest.api.v1.KeepAliveRequest, com.quorum.quest.api.v1.KeepAliveResponse> getKeepAliveMethod;
    if ((getKeepAliveMethod = LeaderElectionServiceGrpc.getKeepAliveMethod) == null) {
      synchronized (LeaderElectionServiceGrpc.class) {
        if ((getKeepAliveMethod = LeaderElectionServiceGrpc.getKeepAliveMethod) == null) {
          LeaderElectionServiceGrpc.getKeepAliveMethod = getKeepAliveMethod =
              io.grpc.MethodDescriptor.<com.quorum.quest.api.v1.KeepAliveRequest, com.quorum.quest.api.v1.KeepAliveResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "KeepAlive"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.quorum.quest.api.v1.KeepAliveRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.quorum.quest.api.v1.KeepAliveResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LeaderElectionServiceMethodDescriptorSupplier("KeepAlive"))
              .build();
        }
      }
    }
    return getKeepAliveMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static LeaderElectionServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LeaderElectionServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LeaderElectionServiceStub>() {
        @java.lang.Override
        public LeaderElectionServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LeaderElectionServiceStub(channel, callOptions);
        }
      };
    return LeaderElectionServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static LeaderElectionServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LeaderElectionServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LeaderElectionServiceBlockingV2Stub>() {
        @java.lang.Override
        public LeaderElectionServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LeaderElectionServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return LeaderElectionServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static LeaderElectionServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LeaderElectionServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LeaderElectionServiceBlockingStub>() {
        @java.lang.Override
        public LeaderElectionServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LeaderElectionServiceBlockingStub(channel, callOptions);
        }
      };
    return LeaderElectionServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static LeaderElectionServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LeaderElectionServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LeaderElectionServiceFutureStub>() {
        @java.lang.Override
        public LeaderElectionServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LeaderElectionServiceFutureStub(channel, callOptions);
        }
      };
    return LeaderElectionServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * LeaderElectionService provides distributed leader election capabilities
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * TryAcquireLock attempts to acquire leadership for a given service/domain
     * </pre>
     */
    default void tryAcquireLock(com.quorum.quest.api.v1.TryAcquireLockRequest request,
        io.grpc.stub.StreamObserver<com.quorum.quest.api.v1.TryAcquireLockResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getTryAcquireLockMethod(), responseObserver);
    }

    /**
     * <pre>
     * ReleaseLock voluntarily releases leadership
     * </pre>
     */
    default void releaseLock(com.quorum.quest.api.v1.ReleaseLockRequest request,
        io.grpc.stub.StreamObserver<com.quorum.quest.api.v1.ReleaseLockResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getReleaseLockMethod(), responseObserver);
    }

    /**
     * <pre>
     * KeepAlive extends the leadership lease
     * </pre>
     */
    default void keepAlive(com.quorum.quest.api.v1.KeepAliveRequest request,
        io.grpc.stub.StreamObserver<com.quorum.quest.api.v1.KeepAliveResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getKeepAliveMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service LeaderElectionService.
   * <pre>
   * LeaderElectionService provides distributed leader election capabilities
   * </pre>
   */
  public static abstract class LeaderElectionServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return LeaderElectionServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service LeaderElectionService.
   * <pre>
   * LeaderElectionService provides distributed leader election capabilities
   * </pre>
   */
  public static final class LeaderElectionServiceStub
      extends io.grpc.stub.AbstractAsyncStub<LeaderElectionServiceStub> {
    private LeaderElectionServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LeaderElectionServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LeaderElectionServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * TryAcquireLock attempts to acquire leadership for a given service/domain
     * </pre>
     */
    public void tryAcquireLock(com.quorum.quest.api.v1.TryAcquireLockRequest request,
        io.grpc.stub.StreamObserver<com.quorum.quest.api.v1.TryAcquireLockResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getTryAcquireLockMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * ReleaseLock voluntarily releases leadership
     * </pre>
     */
    public void releaseLock(com.quorum.quest.api.v1.ReleaseLockRequest request,
        io.grpc.stub.StreamObserver<com.quorum.quest.api.v1.ReleaseLockResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getReleaseLockMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * KeepAlive extends the leadership lease
     * </pre>
     */
    public void keepAlive(com.quorum.quest.api.v1.KeepAliveRequest request,
        io.grpc.stub.StreamObserver<com.quorum.quest.api.v1.KeepAliveResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getKeepAliveMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service LeaderElectionService.
   * <pre>
   * LeaderElectionService provides distributed leader election capabilities
   * </pre>
   */
  public static final class LeaderElectionServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<LeaderElectionServiceBlockingV2Stub> {
    private LeaderElectionServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LeaderElectionServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LeaderElectionServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * TryAcquireLock attempts to acquire leadership for a given service/domain
     * </pre>
     */
    public com.quorum.quest.api.v1.TryAcquireLockResponse tryAcquireLock(com.quorum.quest.api.v1.TryAcquireLockRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getTryAcquireLockMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ReleaseLock voluntarily releases leadership
     * </pre>
     */
    public com.quorum.quest.api.v1.ReleaseLockResponse releaseLock(com.quorum.quest.api.v1.ReleaseLockRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getReleaseLockMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * KeepAlive extends the leadership lease
     * </pre>
     */
    public com.quorum.quest.api.v1.KeepAliveResponse keepAlive(com.quorum.quest.api.v1.KeepAliveRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getKeepAliveMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service LeaderElectionService.
   * <pre>
   * LeaderElectionService provides distributed leader election capabilities
   * </pre>
   */
  public static final class LeaderElectionServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<LeaderElectionServiceBlockingStub> {
    private LeaderElectionServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LeaderElectionServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LeaderElectionServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * TryAcquireLock attempts to acquire leadership for a given service/domain
     * </pre>
     */
    public com.quorum.quest.api.v1.TryAcquireLockResponse tryAcquireLock(com.quorum.quest.api.v1.TryAcquireLockRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getTryAcquireLockMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * ReleaseLock voluntarily releases leadership
     * </pre>
     */
    public com.quorum.quest.api.v1.ReleaseLockResponse releaseLock(com.quorum.quest.api.v1.ReleaseLockRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getReleaseLockMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * KeepAlive extends the leadership lease
     * </pre>
     */
    public com.quorum.quest.api.v1.KeepAliveResponse keepAlive(com.quorum.quest.api.v1.KeepAliveRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getKeepAliveMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service LeaderElectionService.
   * <pre>
   * LeaderElectionService provides distributed leader election capabilities
   * </pre>
   */
  public static final class LeaderElectionServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<LeaderElectionServiceFutureStub> {
    private LeaderElectionServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LeaderElectionServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LeaderElectionServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * TryAcquireLock attempts to acquire leadership for a given service/domain
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.quorum.quest.api.v1.TryAcquireLockResponse> tryAcquireLock(
        com.quorum.quest.api.v1.TryAcquireLockRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getTryAcquireLockMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * ReleaseLock voluntarily releases leadership
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.quorum.quest.api.v1.ReleaseLockResponse> releaseLock(
        com.quorum.quest.api.v1.ReleaseLockRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getReleaseLockMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * KeepAlive extends the leadership lease
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.quorum.quest.api.v1.KeepAliveResponse> keepAlive(
        com.quorum.quest.api.v1.KeepAliveRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getKeepAliveMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_TRY_ACQUIRE_LOCK = 0;
  private static final int METHODID_RELEASE_LOCK = 1;
  private static final int METHODID_KEEP_ALIVE = 2;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_TRY_ACQUIRE_LOCK:
          serviceImpl.tryAcquireLock((com.quorum.quest.api.v1.TryAcquireLockRequest) request,
              (io.grpc.stub.StreamObserver<com.quorum.quest.api.v1.TryAcquireLockResponse>) responseObserver);
          break;
        case METHODID_RELEASE_LOCK:
          serviceImpl.releaseLock((com.quorum.quest.api.v1.ReleaseLockRequest) request,
              (io.grpc.stub.StreamObserver<com.quorum.quest.api.v1.ReleaseLockResponse>) responseObserver);
          break;
        case METHODID_KEEP_ALIVE:
          serviceImpl.keepAlive((com.quorum.quest.api.v1.KeepAliveRequest) request,
              (io.grpc.stub.StreamObserver<com.quorum.quest.api.v1.KeepAliveResponse>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getTryAcquireLockMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.quorum.quest.api.v1.TryAcquireLockRequest,
              com.quorum.quest.api.v1.TryAcquireLockResponse>(
                service, METHODID_TRY_ACQUIRE_LOCK)))
        .addMethod(
          getReleaseLockMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.quorum.quest.api.v1.ReleaseLockRequest,
              com.quorum.quest.api.v1.ReleaseLockResponse>(
                service, METHODID_RELEASE_LOCK)))
        .addMethod(
          getKeepAliveMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.quorum.quest.api.v1.KeepAliveRequest,
              com.quorum.quest.api.v1.KeepAliveResponse>(
                service, METHODID_KEEP_ALIVE)))
        .build();
  }

  private static abstract class LeaderElectionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    LeaderElectionServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.quorum.quest.api.v1.QuorumQuestApiProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("LeaderElectionService");
    }
  }

  private static final class LeaderElectionServiceFileDescriptorSupplier
      extends LeaderElectionServiceBaseDescriptorSupplier {
    LeaderElectionServiceFileDescriptorSupplier() {}
  }

  private static final class LeaderElectionServiceMethodDescriptorSupplier
      extends LeaderElectionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    LeaderElectionServiceMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (LeaderElectionServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new LeaderElectionServiceFileDescriptorSupplier())
              .addMethod(getTryAcquireLockMethod())
              .addMethod(getReleaseLockMethod())
              .addMethod(getKeepAliveMethod())
              .build();
        }
      }
    }
    return result;
  }
}
