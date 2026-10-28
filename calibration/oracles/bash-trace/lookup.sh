# Native namespace inventory/query. No collector functions are introduced.
builtin shopt -s extdebug
__oracle_root=$1
__oracle_loads=$2
__oracle_queries=$3
__oracle_raw=$4
export BATS_ROOT=$__oracle_root BATS_LIBDIR=lib NVM_DIR=$__oracle_root
while IFS= read -r -d '' __oracle_load; do
  if [[ $__oracle_load == nvm.sh ]]; then
    builtin source "$__oracle_root/$__oracle_load" --no-use || exit 82
  else
    builtin source "$__oracle_root/$__oracle_load" || exit 82
  fi
done < "$__oracle_loads"
while IFS= read -r __oracle_name; do
  [[ $__oracle_name =~ ^[a-zA-Z_][a-zA-Z_0-9:.+-]*$ ]] || exit 81
  __oracle_definition=$(builtin declare -F -- "$__oracle_name") || exit 83
  builtin printf 'F\0%s\0%s\0' "$__oracle_name" "$__oracle_definition" >> "$__oracle_raw"
done < <(builtin compgen -A function)
while IFS= read -r __oracle_name; do
  builtin printf 'B\0%s\0\0' "$__oracle_name" >> "$__oracle_raw"
done < <(builtin compgen -b)
while IFS= read -r -d '' __oracle_query; do
  __oracle_kind=$(builtin type -t -- "$__oracle_query" 2>/dev/null) || __oracle_kind=missing
  builtin printf 'Q\0%s\0%s\0' "$__oracle_query" "$__oracle_kind" >> "$__oracle_raw"
done < "$__oracle_queries"
