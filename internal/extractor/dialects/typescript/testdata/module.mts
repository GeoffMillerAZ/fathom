export type Id = string;

export function load(id: Id): Promise<string> {
  return read(id);
}

const cache = new Map<Id, string>();
