"""import stray; __import__('ignored')"""
import app.core as renamed, top
from .core import (
    named as original,
    other,
)
from . import child as picked
from app.core import *
if False: import app.child
import importlib as il
il.import_module("app." + name)
__import__("/private/runtime/input")
