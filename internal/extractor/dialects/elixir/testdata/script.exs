ExUnit.start()

defmodule ScriptTest do
  use ExUnit.Case, async: true

  setup do
    {:ok, n: 1}
  end

  describe "arithmetic" do
    test "adds", %{n: n} do
      assert n + 1 == 2
    end
  end

  test "it works" do
    assert Enum.sum([1]) == 1
  end
end

Mix.install([:jason])
IO.puts(Jason.encode!(%{a: 1}))
