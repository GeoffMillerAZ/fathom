defmodule App.Web do
  alias App.Repo
  alias App.Accounts.User, as: U
  alias App.{Mailer, Billing.Invoice}
  import App.Helpers
  import App.Format, only: [money: 1]
  require App.Log
  use App.Schema
  use App.Case, async: true

  alias __MODULE__.Child
  def page(x), do: money(x)
end
