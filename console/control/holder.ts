import { ControllerKind, type Controller } from '@tourian/contract/tourian/v1/control_pb'

export function ownOperator(holder: Controller | undefined, consoleId: string | undefined): boolean {
  return holder?.kind === ControllerKind.OPERATOR && holder.id === consoleId
}

// Who holds the robot, as this console's operator reads it; brief where there is little room.
export function holderLabel(
  holder: Controller | undefined,
  consoleId: string | undefined,
  { brief = false } = {},
): string {
  if (!holder) return 'nobody'
  if (ownOperator(holder, consoleId)) return 'you'
  if (brief) return holder.kind === ControllerKind.OPERATOR ? 'operator' : holder.name
  switch (holder.kind) {
    case ControllerKind.OPERATOR:
      return 'another operator'
    case ControllerKind.SCRIPT:
      return `script: ${holder.name}`
    case ControllerKind.POLICY:
      return holder.name
    default:
      return holder.name || holder.id
  }
}
