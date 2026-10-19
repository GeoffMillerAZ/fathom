-- Type aliases: plain, generic, exported, function and union types.
export type Point = { x: number, y: number }
type Callback<T> = (value: T) -> ()
type Result<T, E> = { ok: true, value: T } | { ok: false, error: E }
export type Handler = (player: Player) -> ()
type Pair<A, B> = { first: A, second: B }

local function make(): Point
  return { x = 0, y = 0 }
end
