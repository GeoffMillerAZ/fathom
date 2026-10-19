defmodule Before do
  def ok(x), do: x
end

defmodule Broken do
  def bad(x do
    x +
  end
end

defmodule After do
  def fine(y), do: y
end
