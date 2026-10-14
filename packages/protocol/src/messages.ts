import {
  create,
  fromBinary,
  toBinary,
  type MessageInitShape,
  type MessageShape,
} from "@bufbuild/protobuf";
import { encodeFrame, type Frame } from "./framing";
import * as pb from "./gen/Mumble_pb";

// Index in this list is the TCP message type ID.
const SCHEMAS = [
  pb.VersionSchema,
  pb.UDPTunnelSchema,
  pb.AuthenticateSchema,
  pb.PingSchema,
  pb.RejectSchema,
  pb.ServerSyncSchema,
  pb.ChannelRemoveSchema,
  pb.ChannelStateSchema,
  pb.UserRemoveSchema,
  pb.UserStateSchema,
  pb.BanListSchema,
  pb.TextMessageSchema,
  pb.PermissionDeniedSchema,
  pb.ACLSchema,
  pb.QueryUsersSchema,
  pb.CryptSetupSchema,
  pb.ContextActionModifySchema,
  pb.ContextActionSchema,
  pb.UserListSchema,
  pb.VoiceTargetSchema,
  pb.PermissionQuerySchema,
  pb.CodecVersionSchema,
  pb.UserStatsSchema,
  pb.RequestBlobSchema,
  pb.ServerConfigSchema,
  pb.SuggestConfigSchema,
  pb.PluginDataTransmissionSchema,
] as const;

type Schema = (typeof SCHEMAS)[number];
type NameOf<S> = S extends { typeName: `MumbleProto.${infer N}` } ? N : never;
type SchemaByName = { [S in Schema as NameOf<S>]: S };

export type ControlType = Exclude<keyof SchemaByName, "UDPTunnel">;

export type ControlMessage = {
  [K in ControlType]: { type: K; message: MessageShape<SchemaByName[K]> };
}[ControlType];

export type MumbleMessage =
  | ControlMessage
  | { type: "UDPTunnel"; packet: Uint8Array }
  | { type: "Unknown"; id: number; payload: Uint8Array };

const UDP_TUNNEL = 1;
const nameOf = (schema: Schema) => schema.typeName.replace("MumbleProto.", "");
const ID_BY_NAME = new Map(SCHEMAS.map((s, id) => [nameOf(s), id]));

export const encodeMessage = <K extends ControlType>(
  type: K,
  init: MessageInitShape<SchemaByName[K]>,
): Uint8Array => {
  const id = ID_BY_NAME.get(type)!;
  // The mapped types are correct per key, but TS cannot correlate them here.
  const schema = SCHEMAS[id] as typeof pb.VersionSchema;
  return encodeFrame(id, toBinary(schema, create(schema, init as never)));
};

export const encodeVoicePacket = (packet: Uint8Array): Uint8Array =>
  encodeFrame(UDP_TUNNEL, packet);

export const decodeMessage = ({ type: id, payload }: Frame): MumbleMessage => {
  if (id === UDP_TUNNEL) return { type: "UDPTunnel", packet: payload };
  const schema = SCHEMAS[id];
  if (!schema) return { type: "Unknown", id, payload };
  return {
    type: nameOf(schema),
    message: fromBinary(schema as typeof pb.VersionSchema, payload),
  } as ControlMessage;
};
