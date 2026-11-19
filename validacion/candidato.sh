# Sourced by the suites: candidato PATH checks that PATH is the orq that make ci built
# (ORQ_CANDIDATO) and that it was built from this checkout's HEAD.
candidato() {
  local head line
  head=$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse HEAD) || return 1
  if [ -z "${ORQ_CANDIDATO:-}" ]; then
    echo "orq under test is not the candidate: ORQ_CANDIDATO is not set (run make ci)"; return 1
  fi
  if [ "$1" != "$ORQ_CANDIDATO" ]; then
    echo "orq under test is not the candidate: $1, expected $ORQ_CANDIDATO"; return 1
  fi
  line=$("$1" version)
  echo "candidate: $1 · $line"
  case " $line " in
    *" revision=$head "*) ;;
    *) echo "orq under test is not the candidate: built from another revision, expected revision=$head"; return 1 ;;
  esac
}
