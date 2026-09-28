export function exported() {
  return internal();
}

function internal() {
  return 42;
}

export const arrowExport = () => exported();
