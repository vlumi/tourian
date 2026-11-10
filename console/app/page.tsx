import {
  BatterySchema,
  CapabilitiesSchema,
  HeartbeatSchema,
  JointStateSchema,
  ObservationSchema,
  StopSchema,
  VelocityCommandSchema,
} from '@tourian/contract/tourian/v1/robot_pb'

const contract = [
  CapabilitiesSchema,
  VelocityCommandSchema,
  ObservationSchema,
  JointStateSchema,
  BatterySchema,
  HeartbeatSchema,
  StopSchema,
]

export default function Page() {
  return (
    <main>
      <h1>tourian</h1>
      <p>A control room for robots, and for whatever is at the controls.</p>
      <h2>Contract</h2>
      <ul>
        {contract.map((schema) => (
          <li key={schema.typeName}>
            {schema.typeName}
            <span>{schema.fields.length} fields</span>
          </li>
        ))}
      </ul>
    </main>
  )
}
