"""Isolated subject copy preserving pinned symlinks and private Git metadata."""
import shutil


def copy_subject(source,destination):
    git=source/'.git'
    if git.exists() and not git.is_dir():
        raise ValueError('whole-unit refusal: shared worktree Git metadata')
    if git.is_symlink():
        raise ValueError('whole-unit refusal: symlinked Git metadata')
    shutil.copytree(source,destination,symlinks=True,ignore=shutil.ignore_patterns('node_modules'))


def runtime_environment(cache, inherited):
    # Both traced and untraced drivers use the same writable scratch contract.
    result={k:v for k,v in inherited.items() if not k.startswith(('BATS_','BASH_FUNC_','GIT_CONFIG_','GIT_AUTHOR_','GIT_COMMITTER_','NVM_')) and not k.lower().startswith('npm_config_') and k not in ['PROFILE','SHELLOPTS','BASHOPTS','CDPATH','ENV']}
    home=cache/'home'
    home.mkdir(parents=True,exist_ok=True)
    npm=cache/'npm'
    npm.mkdir(parents=True,exist_ok=True)
    result.update(HOME=str(home),npm_config_cache=str(npm),GIT_CONFIG_NOSYSTEM='1')
    scratch=cache/'tmp'
    scratch.mkdir(parents=True,exist_ok=True)
    for key in ['TMPDIR','TMP','TEMP']:
        result[key]=str(scratch)
    for key,name in [('XDG_CACHE_HOME','xdg-cache'),('XDG_CONFIG_HOME','xdg-config'),('XDG_DATA_HOME','xdg-data'),('XDG_STATE_HOME','xdg-state')]:
        path=cache/name
        path.mkdir(parents=True,exist_ok=True)
        result[key]=str(path)
    return result
