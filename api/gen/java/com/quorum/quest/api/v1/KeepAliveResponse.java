// Generated by the protocol buffer compiler.  DO NOT EDIT!
// NO CHECKED-IN PROTOBUF GENCODE
// source: v1/quorum_quest_api.proto
// Protobuf Java Version: 4.29.3

package com.quorum.quest.api.v1;

/**
 * Protobuf type {@code quorum.quest.api.v1.KeepAliveResponse}
 */
public  final class KeepAliveResponse extends
    com.google.protobuf.GeneratedMessageLite<
        KeepAliveResponse, KeepAliveResponse.Builder> implements
    // @@protoc_insertion_point(message_implements:quorum.quest.api.v1.KeepAliveResponse)
    KeepAliveResponseOrBuilder {
  private KeepAliveResponse() {
  }
  private int bitField0_;
  public static final int LEASE_LENGTH_FIELD_NUMBER = 1;
  private com.google.protobuf.Duration leaseLength_;
  /**
   * <pre>
   * Duration of the new lease
   * </pre>
   *
   * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
   */
  @java.lang.Override
  public boolean hasLeaseLength() {
    return ((bitField0_ & 0x00000001) != 0);
  }
  /**
   * <pre>
   * Duration of the new lease
   * </pre>
   *
   * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
   */
  @java.lang.Override
  public com.google.protobuf.Duration getLeaseLength() {
    return leaseLength_ == null ? com.google.protobuf.Duration.getDefaultInstance() : leaseLength_;
  }
  /**
   * <pre>
   * Duration of the new lease
   * </pre>
   *
   * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
   */
  @java.lang.SuppressWarnings("ReturnValueIgnored")
  private void setLeaseLength(com.google.protobuf.Duration value) {
    value.getClass();  // minimal bytecode null check
    leaseLength_ = value;
    bitField0_ |= 0x00000001;
    }
  /**
   * <pre>
   * Duration of the new lease
   * </pre>
   *
   * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
   */
  @java.lang.SuppressWarnings({"ReferenceEquality", "ReturnValueIgnored"})
  private void mergeLeaseLength(com.google.protobuf.Duration value) {
    value.getClass();  // minimal bytecode null check
    if (leaseLength_ != null &&
        leaseLength_ != com.google.protobuf.Duration.getDefaultInstance()) {
      leaseLength_ =
        com.google.protobuf.Duration.newBuilder(leaseLength_).mergeFrom(value).buildPartial();
    } else {
      leaseLength_ = value;
    }
    bitField0_ |= 0x00000001;
  }
  /**
   * <pre>
   * Duration of the new lease
   * </pre>
   *
   * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
   */
  private void clearLeaseLength() {  leaseLength_ = null;
    bitField0_ = (bitField0_ & ~0x00000001);
  }

  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(
      java.nio.ByteBuffer data)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data);
  }
  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(
      java.nio.ByteBuffer data,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data, extensionRegistry);
  }
  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(
      com.google.protobuf.ByteString data)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data);
  }
  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(
      com.google.protobuf.ByteString data,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data, extensionRegistry);
  }
  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(byte[] data)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data);
  }
  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(
      byte[] data,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data, extensionRegistry);
  }
  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(java.io.InputStream input)
      throws java.io.IOException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, input);
  }
  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(
      java.io.InputStream input,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws java.io.IOException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, input, extensionRegistry);
  }

  public static com.quorum.quest.api.v1.KeepAliveResponse parseDelimitedFrom(java.io.InputStream input)
      throws java.io.IOException {
    return parseDelimitedFrom(DEFAULT_INSTANCE, input);
  }

  public static com.quorum.quest.api.v1.KeepAliveResponse parseDelimitedFrom(
      java.io.InputStream input,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws java.io.IOException {
    return parseDelimitedFrom(DEFAULT_INSTANCE, input, extensionRegistry);
  }
  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(
      com.google.protobuf.CodedInputStream input)
      throws java.io.IOException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, input);
  }
  public static com.quorum.quest.api.v1.KeepAliveResponse parseFrom(
      com.google.protobuf.CodedInputStream input,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws java.io.IOException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, input, extensionRegistry);
  }

  public static Builder newBuilder() {
    return (Builder) DEFAULT_INSTANCE.createBuilder();
  }
  public static Builder newBuilder(com.quorum.quest.api.v1.KeepAliveResponse prototype) {
    return DEFAULT_INSTANCE.createBuilder(prototype);
  }

  /**
   * Protobuf type {@code quorum.quest.api.v1.KeepAliveResponse}
   */
  public static final class Builder extends
      com.google.protobuf.GeneratedMessageLite.Builder<
        com.quorum.quest.api.v1.KeepAliveResponse, Builder> implements
      // @@protoc_insertion_point(builder_implements:quorum.quest.api.v1.KeepAliveResponse)
      com.quorum.quest.api.v1.KeepAliveResponseOrBuilder {
    // Construct using com.quorum.quest.api.v1.KeepAliveResponse.newBuilder()
    private Builder() {
      super(DEFAULT_INSTANCE);
    }


    /**
     * <pre>
     * Duration of the new lease
     * </pre>
     *
     * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
     */
    @java.lang.Override
    public boolean hasLeaseLength() {
      return instance.hasLeaseLength();
    }
    /**
     * <pre>
     * Duration of the new lease
     * </pre>
     *
     * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
     */
    @java.lang.Override
    public com.google.protobuf.Duration getLeaseLength() {
      return instance.getLeaseLength();
    }
    /**
     * <pre>
     * Duration of the new lease
     * </pre>
     *
     * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
     */
    public Builder setLeaseLength(com.google.protobuf.Duration value) {
      copyOnWrite();
      instance.setLeaseLength(value);
      return this;
      }
    /**
     * <pre>
     * Duration of the new lease
     * </pre>
     *
     * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
     */
    public Builder setLeaseLength(
        com.google.protobuf.Duration.Builder builderForValue) {
      copyOnWrite();
      instance.setLeaseLength(builderForValue.build());
      return this;
    }
    /**
     * <pre>
     * Duration of the new lease
     * </pre>
     *
     * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
     */
    public Builder mergeLeaseLength(com.google.protobuf.Duration value) {
      copyOnWrite();
      instance.mergeLeaseLength(value);
      return this;
    }
    /**
     * <pre>
     * Duration of the new lease
     * </pre>
     *
     * <code>.google.protobuf.Duration lease_length = 1 [json_name = "leaseLength"];</code>
     */
    public Builder clearLeaseLength() {  copyOnWrite();
      instance.clearLeaseLength();
      return this;
    }

    // @@protoc_insertion_point(builder_scope:quorum.quest.api.v1.KeepAliveResponse)
  }
  @java.lang.Override
  @java.lang.SuppressWarnings({"unchecked", "fallthrough"})
  protected final java.lang.Object dynamicMethod(
      com.google.protobuf.GeneratedMessageLite.MethodToInvoke method,
      java.lang.Object arg0, java.lang.Object arg1) {
    switch (method) {
      case NEW_MUTABLE_INSTANCE: {
        return new com.quorum.quest.api.v1.KeepAliveResponse();
      }
      case NEW_BUILDER: {
        return new Builder();
      }
      case BUILD_MESSAGE_INFO: {
          java.lang.Object[] objects = new java.lang.Object[] {
            "bitField0_",
            "leaseLength_",
          };
          java.lang.String info =
              "\u0000\u0001\u0000\u0001\u0001\u0001\u0001\u0000\u0000\u0000\u0001\u1009\u0000";
          return newMessageInfo(DEFAULT_INSTANCE, info, objects);
      }
      // fall through
      case GET_DEFAULT_INSTANCE: {
        return DEFAULT_INSTANCE;
      }
      case GET_PARSER: {
        com.google.protobuf.Parser<com.quorum.quest.api.v1.KeepAliveResponse> parser = PARSER;
        if (parser == null) {
          synchronized (com.quorum.quest.api.v1.KeepAliveResponse.class) {
            parser = PARSER;
            if (parser == null) {
              parser =
                  new DefaultInstanceBasedParser<com.quorum.quest.api.v1.KeepAliveResponse>(
                      DEFAULT_INSTANCE);
              PARSER = parser;
            }
          }
        }
        return parser;
    }
    case GET_MEMOIZED_IS_INITIALIZED: {
      return (byte) 1;
    }
    case SET_MEMOIZED_IS_INITIALIZED: {
      return null;
    }
    }
    throw new UnsupportedOperationException();
  }


  // @@protoc_insertion_point(class_scope:quorum.quest.api.v1.KeepAliveResponse)
  private static final com.quorum.quest.api.v1.KeepAliveResponse DEFAULT_INSTANCE;
  static {
    KeepAliveResponse defaultInstance = new KeepAliveResponse();
    // New instances are implicitly immutable so no need to make
    // immutable.
    DEFAULT_INSTANCE = defaultInstance;
    com.google.protobuf.GeneratedMessageLite.registerDefaultInstance(
      KeepAliveResponse.class, defaultInstance);
  }

  public static com.quorum.quest.api.v1.KeepAliveResponse getDefaultInstance() {
    return DEFAULT_INSTANCE;
  }

  private static volatile com.google.protobuf.Parser<KeepAliveResponse> PARSER;

  public static com.google.protobuf.Parser<KeepAliveResponse> parser() {
    return DEFAULT_INSTANCE.getParserForType();
  }
}

