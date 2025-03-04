# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: v1/quorum_quest_api.proto
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from google.protobuf import duration_pb2 as google_dot_protobuf_dot_duration__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x19v1/quorum_quest_api.proto\x12\x13quorum.quest.api.v1\x1a\x1egoogle/protobuf/duration.proto\"x\n\x15TryAcquireLockRequest\x12\x18\n\x07service\x18\x01 \x01(\tR\x07service\x12\x16\n\x06\x64omain\x18\x02 \x01(\tR\x06\x64omain\x12\x1b\n\tclient_id\x18\x03 \x01(\tR\x08\x63lientId\x12\x10\n\x03ttl\x18\x04 \x01(\x05R\x03ttl\"5\n\x16TryAcquireLockResponse\x12\x1b\n\tis_leader\x18\x01 \x01(\x08R\x08isLeader\"c\n\x12ReleaseLockRequest\x12\x18\n\x07service\x18\x01 \x01(\tR\x07service\x12\x16\n\x06\x64omain\x18\x02 \x01(\tR\x06\x64omain\x12\x1b\n\tclient_id\x18\x03 \x01(\tR\x08\x63lientId\"\x15\n\x13ReleaseLockResponse\"s\n\x10KeepAliveRequest\x12\x18\n\x07service\x18\x01 \x01(\tR\x07service\x12\x16\n\x06\x64omain\x18\x02 \x01(\tR\x06\x64omain\x12\x1b\n\tclient_id\x18\x03 \x01(\tR\x08\x63lientId\x12\x10\n\x03ttl\x18\x04 \x01(\x05R\x03ttl\"Q\n\x11KeepAliveResponse\x12<\n\x0clease_length\x18\x01 \x01(\x0b\x32\x19.google.protobuf.DurationR\x0bleaseLength2\xc6\x02\n\x15LeaderElectionService\x12k\n\x0eTryAcquireLock\x12*.quorum.quest.api.v1.TryAcquireLockRequest\x1a+.quorum.quest.api.v1.TryAcquireLockResponse\"\x00\x12\x62\n\x0bReleaseLock\x12\'.quorum.quest.api.v1.ReleaseLockRequest\x1a(.quorum.quest.api.v1.ReleaseLockResponse\"\x00\x12\\\n\tKeepAlive\x12%.quorum.quest.api.v1.KeepAliveRequest\x1a&.quorum.quest.api.v1.KeepAliveResponse\"\x00\x42\xd0\x01\n\x17\x63om.quorum.quest.api.v1B\x13QuorumQuestApiProtoP\x01Z1github.com/avivl/quorum-quest/api/gen/go/v1;apiv1\xa2\x02\x03QQA\xaa\x02\x13Quorum.Quest.Api.V1\xca\x02\x13Quorum\\Quest\\Api\\V1\xe2\x02\x1fQuorum\\Quest\\Api\\V1\\GPBMetadata\xea\x02\x16Quorum::Quest::Api::V1b\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'v1.quorum_quest_api_pb2', _globals)
if _descriptor._USE_C_DESCRIPTORS == False:
  _globals['DESCRIPTOR']._options = None
  _globals['DESCRIPTOR']._serialized_options = b'\n\027com.quorum.quest.api.v1B\023QuorumQuestApiProtoP\001Z1github.com/avivl/quorum-quest/api/gen/go/v1;apiv1\242\002\003QQA\252\002\023Quorum.Quest.Api.V1\312\002\023Quorum\\Quest\\Api\\V1\342\002\037Quorum\\Quest\\Api\\V1\\GPBMetadata\352\002\026Quorum::Quest::Api::V1'
  _globals['_TRYACQUIRELOCKREQUEST']._serialized_start=82
  _globals['_TRYACQUIRELOCKREQUEST']._serialized_end=202
  _globals['_TRYACQUIRELOCKRESPONSE']._serialized_start=204
  _globals['_TRYACQUIRELOCKRESPONSE']._serialized_end=257
  _globals['_RELEASELOCKREQUEST']._serialized_start=259
  _globals['_RELEASELOCKREQUEST']._serialized_end=358
  _globals['_RELEASELOCKRESPONSE']._serialized_start=360
  _globals['_RELEASELOCKRESPONSE']._serialized_end=381
  _globals['_KEEPALIVEREQUEST']._serialized_start=383
  _globals['_KEEPALIVEREQUEST']._serialized_end=498
  _globals['_KEEPALIVERESPONSE']._serialized_start=500
  _globals['_KEEPALIVERESPONSE']._serialized_end=581
  _globals['_LEADERELECTIONSERVICE']._serialized_start=584
  _globals['_LEADERELECTIONSERVICE']._serialized_end=910
# @@protoc_insertion_point(module_scope)
