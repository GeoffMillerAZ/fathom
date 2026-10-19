defmodule Calls do
  alias Calls.Util

  def run(conn, items) do
    items
    |> Enum.map(&format/1)
    |> Util.join()
    |> finish
    conn.assigns
    Calls.Util.helper(1)
    Util.helper(2)
    handler.(items)
    fetch(items).body
    if items == [], do: raise("none"), else: send(self(), :done)
    case items do
      [] -> log(:empty)
      _ -> :ok
    end
    :erlang.hash(items, 7)
    Map.get(%{}, :a)
  end

  defp format(i), do: to_string(i)
  defp finish(x), do: x
  defp log(x), do: IO.puts(x)
  defp fetch(x), do: x
end
