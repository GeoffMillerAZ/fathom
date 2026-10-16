#!/bin/bash
# IXS-M0 cache-from timing harness. Method: cache-from-timing.md (fixed before data, K19).
# usage: cache-from-timing.sh run        # whole program, resumable (results.tsv decides what is left)
#        cache-from-timing.sh summary    # per-commit table + medians from results.tsv
# Env: BUILD_ROOT (the wave build root: scripts/genv-wave.sh, waves/, w-<label>/; required),
#      SRC_REPO (a clone holding the commits), TIP (default 99aa205a), WLABEL (default war2-timing).
set -u
B=${BUILD_ROOT:?set BUILD_ROOT to the wave build root}
WLABEL=${WLABEL:-war2-timing}
W=$B/w-$WLABEL
R=$W/run
BIN=$W/bin/fathom
RES=$R/results.tsv
ILOCK=$B/waves/war2/ingest.lock
TLOCK=$B/waves/war2/test.lock
TIP=${TIP:-99aa205a}
SRC_REPO=${SRC_REPO:-$B/waves/war2/timing}
MINFREE=15
SELF=$(cd "$(dirname "$0")" && pwd)/$(basename "$0")
mkdir -p "$R"

env_setup() {
  # shellcheck disable=SC1091
  source "$B/scripts/genv-wave.sh" "$WLABEL"
  export FATHOM_THREADS=$B/wave-maps/threads.jsonl
}

pressure() { "$BIN" resources 2>/dev/null | awk '/^host/ {for(i=1;i<=NF;i++) if($i=="pressure"){print $(i+1); exit}}'; }
swapfree() { df -g / | awk 'NR==2{print $4}'; }

wait_ok() {
  while :; do
    local f p; f=$(swapfree); p=$(pressure); p=${p:-unknown}
    if [ "${f:-0}" -ge "$MINFREE" ] && { [ "$p" = normal ] || [ "$p" = unknown ]; }; then return 0; fi
    echo "$(date +%T) pause: swap-volume free ${f} GiB, pressure ${p}" >&2
    sleep 60
  done
}

commits() { git -C "$R/mirror.git" log --first-parent --format=%H -21 "$TIP" | tail -r; }

have() { [ -f "$RES" ] && awk -F'\t' -v i="$1" -v m="$2" '$1==i && $3==m {f=1} END{exit !f}' "$RES"; }

# timed_ingest IDX SHA MODE STORE CHECKOUT POS PREP_S
timed_ingest() {
  local idx=$1 sha=$2 mode=$3 store=$4 co=$5 pos=$6 prep=$7
  wait_ok
  local tf=$R/time.$mode.out lg=$R/ingest.$mode.log
  lockf -k "$ILOCK" /usr/bin/time -l -o "$tf" env GOPROXY=off "$BIN" ingest --store "$store" --service fathom --path "$co" >"$lg" 2>&1
  local rc=$?
  local wall rss deg sb ld pr sf
  wall=$(awk '/ real /{print $1; exit}' "$tf")
  rss=$(awk '/maximum resident set size/{print $1; exit}' "$tf")
  deg=0; grep -q 'error: ingest degraded:' "$lg" && deg=1
  sb=$(stat -f %z "$store"* 2>/dev/null | awk '{s+=$1} END{print s+0}')
  ld=$(uptime | sed 's/.*load averages*: //')
  pr=$(pressure); sf=$(swapfree)
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$idx" "$sha" "$mode" "$pos" "$rc" "$deg" "$wall" "$rss" "$sb" "$ld" "${pr:-?}" "$sf" >>"$RES"
  # prep seconds kept in a side file (cache-from only)
  printf '%s\t%s\t%s\n' "$idx" "$mode" "$prep" >>"$R/prep.tsv"
  cp "$lg" "$R/logs/ingest.$idx.$mode.log" 2>/dev/null
  echo "$(date +%T) idx=$idx $mode rc=$rc deg=$deg wall=${wall}s rss=$rss" >&2
}

clone_at() { # DIR SHA
  rm -rf "$1"; git clone -q --no-hardlinks "$R/mirror.git" "$1" && git -C "$1" checkout -q "$2"
}

do_cold() { # idx sha pos
  local i=$1 s=$2 p=$3
  have "$i" cold && return
  rm -rf "$R/co-cold-$i" "$R/cold-$i.sqlite"*
  clone_at "$R/co-cold-$i" "$s" || { echo "clone failed" >&2; return; }
  timed_ingest "$i" "$s" cold "$R/cold-$i.sqlite" "$R/co-cold-$i" "$p" 0
}
do_warm() {
  local i=$1 s=$2 p=$3
  have "$i" warm && return
  git -C "$R/co-warm" checkout -q "$s" || { echo "warm checkout failed" >&2; return; }
  timed_ingest "$i" "$s" warm "$R/warm.sqlite" "$R/co-warm" "$p" 0
}
do_cf() {
  local i=$1 s=$2 p=$3 prev=$((i-1))
  have "$i" cachefrom && return
  if [ ! -f "$R/cold-$prev.sqlite" ] || ! awk -F'\t' -v i="$prev" '$1==i && $3=="cold" && $5==0 && $6==0 {f=1} END{exit !f}' "$RES"; then
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$i" "$s" cachefrom "$p" skip 0 "" "" "" "" "" "" >>"$RES"; return
  fi
  rm -rf "$R/co-cf-$i" "$R/cf-$i.sqlite"*
  clone_at "$R/co-cf-$i" "$s" || { echo "clone failed" >&2; return; }
  local t0=$SECONDS f
  for f in "$R/cold-$prev.sqlite"*; do cp "$f" "$R/cf-$i.sqlite${f#"$R/cold-$prev.sqlite"}"; done
  "$BIN" remap --store "$R/cf-$i.sqlite" --service fathom --path "$R/co-cf-$i" >>"$R/remap.log" 2>&1 || echo "remap failed idx=$i" >&2
  timed_ingest "$i" "$s" cachefrom "$R/cf-$i.sqlite" "$R/co-cf-$i" "$p" $((SECONDS-t0))
}

triple() { # idx : run the three modes for commit idx in the rotated order
  env_setup
  local i=$1 s; s=$(commits | sed -n "$((i+1))p")
  export GOPROXY=off
  local order
  case $((i%3)) in
    1) order="cold warm cachefrom";;
    2) order="warm cachefrom cold";;
    0) order="cachefrom cold warm";;
  esac
  local pos=0 m
  for m in $order; do
    pos=$((pos+1))
    case $m in cold) do_cold "$i" "$s" "$pos";; warm) do_warm "$i" "$s" "$pos";; cachefrom) do_cf "$i" "$s" "$pos";; esac
  done
  # keep only this commit's cold store/checkout as the next base; drop older
  local old=$((i-1))
  rm -rf "$R/co-cold-$old" "$R/cold-$old.sqlite"* "$R/co-cf-$i" "$R/cf-$i.sqlite"* "$R/co-cf-$old" "$R/cf-$old.sqlite"* "$R/co-cold-$i"
}

seed() {
  env_setup
  local s; s=$(commits | sed -n 1p)
  mkdir -p "$R/logs"
  if ! have 0 cold; then
    rm -rf "$R/co-cold-0" "$R/cold-0.sqlite"*
    clone_at "$R/co-cold-0" "$s"
    # first seed run may fetch modules (warming GOMODCACHE); the rest run with GOPROXY=off
    timed_ingest 0 "$s" cold "$R/cold-0.sqlite" "$R/co-cold-0" 0 0
  fi
  if ! have 0 warm; then
    rm -rf "$R/co-warm" "$R/warm.sqlite"*
    clone_at "$R/co-warm" "$s"
    timed_ingest 0 "$s" warm "$R/warm.sqlite" "$R/co-warm" 0 0
  fi
}

run() {
  env_setup
  mkdir -p "$R/logs"
  [ -d "$R/mirror.git" ] || git clone -q --bare "$SRC_REPO" "$R/mirror.git"
  [ -f "$RES" ] || printf 'idx\tsha\tmode\tpos\texit\tdegraded\twall_s\tmax_rss_b\tstore_b\tload\tpressure\tswap_free_gib\n' >"$RES"
  {
    echo "binary: $("$BIN" version --json)"
    echo "go: $(go version)  GOCACHE=$GOCACHE ($(du -sk "$GOCACHE" 2>/dev/null | awk '{print $1/1048576" GiB"}'))  GOMODCACHE=$GOMODCACHE"
    echo "host: $(sw_vers -productVersion) $(uname -m) cores=$(sysctl -n hw.ncpu)"
    commits | awk '{print NR-1, $1}'
  } >"$R/run-env.txt"
  seed
  local i
  for i in $(seq 1 20); do
    if have "$i" cold && have "$i" warm && have "$i" cachefrom; then continue; fi
    wait_ok
    lockf -k "$TLOCK" bash "$SELF" triple "$i"
  done
  echo "$(date +%T) all done" >&2
}

summary() {
  awk -F'\t' '
  function med(a,n,  i,j,t,b){ for(i=1;i<=n;i++)b[i]=a[i]; for(i=1;i<=n;i++)for(j=i+1;j<=n;j++)if(b[j]<b[i]){t=b[i];b[i]=b[j];b[j]=t}; if(n==0)return "NA"; return (n%2)?b[(n+1)/2]:(b[n/2]+b[n/2+1])/2 }
  NR>1 && $1>0 { ok=($5==0 && $6==0); k=$1; sha[k]=substr($2,1,8); st[k,$3]=ok?$7:"FAIL"; if(!ok) st[k,$3]= ($5=="skip")?"skip":"FAIL(" $5 "/" $6 ")"; if(ok) v[k,$3]=$7; if(k>mx)mx=k }
  END{
    print "| # | commit | cold s | warm s | cache-from s | cf/cold | cf/warm |"; print "|--:|---|--:|--:|--:|--:|--:|"
    for(k=1;k<=mx;k++){
      c=v[k,"cold"]; w=v[k,"warm"]; f=v[k,"cachefrom"]
      rc=(c!=""&&f!="")?sprintf("%.3f",f/c):"-"; rw=(w!=""&&f!="")?sprintf("%.3f",f/w):"-"
      printf "| %d | %s | %s | %s | %s | %s | %s |\n",k,sha[k],(c!=""?c:st[k,"cold"]),(w!=""?w:st[k,"warm"]),(f!=""?f:st[k,"cachefrom"]),rc,rw
      if(c!=""){nc++;C[nc]=c}; if(w!=""){nw++;W[nw]=w}; if(f!=""){nf++;F[nf]=f}
      if(c!=""&&f!=""){nrc++;RC[nrc]=f/c}; if(w!=""&&f!=""){nrw++;RW[nrw]=f/w}
    }
    printf "\nmedian cold %s s (n=%d), warm %s s (n=%d), cache-from %s s (n=%d)\n",med(C,nc),nc,med(W,nw),nw,med(F,nf),nf
    printf "median of per-commit cf/cold %s (n=%d), cf/warm %s (n=%d)\n",med(RC,nrc),nrc,med(RW,nrw),nrw
    mc=med(C,nc); mf=med(F,nf); mw=med(W,nw)
    if(mc!="NA"&&mf!="NA") printf "ratio of medians: cache-from/cold %.3f, cache-from/warm %.3f\n", mf/mc, mf/mw
  }' "$RES"
}

case "${1:-}" in
  run) run;;
  triple) triple "$2";;
  summary) summary;;
  *) echo "usage: $0 run|summary" >&2; exit 2;;
esac
