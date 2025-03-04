// Generated by the protocol buffer compiler.  DO NOT EDIT!
// NO CHECKED-IN PROTOBUF GENCODE
// source: v1/quorum_quest_api.proto
// Protobuf Java Version: 4.29.3

package com.quorum.quest.api.v1;

/**
 * <pre>
 * Empty response as the operation is fire-and-forget
 * </pre>
 *
 * Protobuf type {@code quorum.quest.api.v1.ReleaseLockResponse}
 */
public  final class ReleaseLockResponse extends
    com.google.protobuf.GeneratedMessageLite<
        ReleaseLockResponse, ReleaseLockResponse.Builder> implements
    // @@protoc_insertion_point(message_implements:quorum.quest.api.v1.ReleaseLockResponse)
    ReleaseLockResponseOrBuilder {
  private ReleaseLockResponse() {
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(
      java.nio.ByteBuffer data)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data);
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(
      java.nio.ByteBuffer data,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data, extensionRegistry);
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(
      com.google.protobuf.ByteString data)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data);
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(
      com.google.protobuf.ByteString data,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data, extensionRegistry);
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(byte[] data)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data);
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(
      byte[] data,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws com.google.protobuf.InvalidProtocolBufferException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, data, extensionRegistry);
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(java.io.InputStream input)
      throws java.io.IOException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, input);
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(
      java.io.InputStream input,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws java.io.IOException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, input, extensionRegistry);
  }

  public static com.quorum.quest.api.v1.ReleaseLockResponse parseDelimitedFrom(java.io.InputStream input)
      throws java.io.IOException {
    return parseDelimitedFrom(DEFAULT_INSTANCE, input);
  }

  public static com.quorum.quest.api.v1.ReleaseLockResponse parseDelimitedFrom(
      java.io.InputStream input,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws java.io.IOException {
    return parseDelimitedFrom(DEFAULT_INSTANCE, input, extensionRegistry);
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(
      com.google.protobuf.CodedInputStream input)
      throws java.io.IOException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, input);
  }
  public static com.quorum.quest.api.v1.ReleaseLockResponse parseFrom(
      com.google.protobuf.CodedInputStream input,
      com.google.protobuf.ExtensionRegistryLite extensionRegistry)
      throws java.io.IOException {
    return com.google.protobuf.GeneratedMessageLite.parseFrom(
        DEFAULT_INSTANCE, input, extensionRegistry);
  }

  public static Builder newBuilder() {
    return (Builder) DEFAULT_INSTANCE.createBuilder();
  }
  public static Builder newBuilder(com.quorum.quest.api.v1.ReleaseLockResponse prototype) {
    return DEFAULT_INSTANCE.createBuilder(prototype);
  }

  /**
   * <pre>
   * Empty response as the operation is fire-and-forget
   * </pre>
   *
   * Protobuf type {@code quorum.quest.api.v1.ReleaseLockResponse}
   */
  public static final class Builder extends
      com.google.protobuf.GeneratedMessageLite.Builder<
        com.quorum.quest.api.v1.ReleaseLockResponse, Builder> implements
      // @@protoc_insertion_point(builder_implements:quorum.quest.api.v1.ReleaseLockResponse)
      com.quorum.quest.api.v1.ReleaseLockResponseOrBuilder {
    // Construct using com.quorum.quest.api.v1.ReleaseLockResponse.newBuilder()
    private Builder() {
      super(DEFAULT_INSTANCE);
    }


    // @@protoc_insertion_point(builder_scope:quorum.quest.api.v1.ReleaseLockResponse)
  }
  @java.lang.Override
  @java.lang.SuppressWarnings({"unchecked", "fallthrough"})
  protected final java.lang.Object dynamicMethod(
      com.google.protobuf.GeneratedMessageLite.MethodToInvoke method,
      java.lang.Object arg0, java.lang.Object arg1) {
    switch (method) {
      case NEW_MUTABLE_INSTANCE: {
        return new com.quorum.quest.api.v1.ReleaseLockResponse();
      }
      case NEW_BUILDER: {
        return new Builder();
      }
      case BUILD_MESSAGE_INFO: {
          java.lang.Object[] objects = null;java.lang.String info =
              "\u0000\u0000";
          return newMessageInfo(DEFAULT_INSTANCE, info, objects);
      }
      // fall through
      case GET_DEFAULT_INSTANCE: {
        return DEFAULT_INSTANCE;
      }
      case GET_PARSER: {
        com.google.protobuf.Parser<com.quorum.quest.api.v1.ReleaseLockResponse> parser = PARSER;
        if (parser == null) {
          synchronized (com.quorum.quest.api.v1.ReleaseLockResponse.class) {
            parser = PARSER;
            if (parser == null) {
              parser =
                  new DefaultInstanceBasedParser<com.quorum.quest.api.v1.ReleaseLockResponse>(
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


  // @@protoc_insertion_point(class_scope:quorum.quest.api.v1.ReleaseLockResponse)
  private static final com.quorum.quest.api.v1.ReleaseLockResponse DEFAULT_INSTANCE;
  static {
    ReleaseLockResponse defaultInstance = new ReleaseLockResponse();
    // New instances are implicitly immutable so no need to make
    // immutable.
    DEFAULT_INSTANCE = defaultInstance;
    com.google.protobuf.GeneratedMessageLite.registerDefaultInstance(
      ReleaseLockResponse.class, defaultInstance);
  }

  public static com.quorum.quest.api.v1.ReleaseLockResponse getDefaultInstance() {
    return DEFAULT_INSTANCE;
  }

  private static volatile com.google.protobuf.Parser<ReleaseLockResponse> PARSER;

  public static com.google.protobuf.Parser<ReleaseLockResponse> parser() {
    return DEFAULT_INSTANCE.getParserForType();
  }
}

