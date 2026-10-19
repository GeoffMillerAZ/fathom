defprotocol Size do
  @doc "Bytes in a value"
  def size(data)
end

defimpl Size, for: BitString do
  def size(s), do: byte_size(s)
end

defmodule MyError do
  defexception [:message]
end

defmodule Shop.Cart do
  @moduledoc false
  @type t :: map()
  @typep line(a) :: [a]
  @opaque secret :: term()
  @callback total(t) :: integer()
  @macrocallback wrap(term) :: term()
  @callback version :: String.t()

  defguard is_empty(c) when c == []
  defguardp is_full(c) when length(c) > 9
  defmacro unless_empty(c, do: block), do: quote(do: unquote(block))
  defmacrop internal(x), do: x
  defdelegate count(c), to: Enum
  defdelegate size(c), to: Enum, as: :count

  def add(cart, item, qty \\ 1) when is_integer(qty) do
    put(cart, item, qty)
  end

  def add(cart, _item, 0), do: cart

  defp put(cart, item, qty), do: [{item, qty} | cart]

  def zero, do: 0

  def checkout(
        cart,
        user,
        opts \\ []
      ) do
    {:ok, cart, user, opts}
  end

  defmodule Line do
    defstruct [:sku]

    def sku(%__MODULE__{sku: sku}), do: sku
  end
end
