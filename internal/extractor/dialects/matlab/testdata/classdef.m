classdef Account < handle
    % ACCOUNT  A bank account.
    properties
        Balance = 0;
        Owner
    end
    properties (Constant)
        RATE = 0.05;
    end
    properties (SetAccess = private, Dependent)
        Secret
    end
    methods
        function obj = Account(owner, varargin)
            obj.Owner = owner;
            obj.audit();
        end
        function deposit(obj, amount)
            % DEPOSIT  Add funds.
            obj.Balance = obj.Balance + amount;
        end
        function r = get.Secret(obj)
            r = obj.Balance * Account.RATE;
        end
        function set.Secret(obj, value)
            obj.Balance = value;
        end
    end
    methods (Static)
        function a = make(owner)
            a = Account(owner);
        end
    end
    methods (Access = private)
        function audit(obj)
        end
    end
    events
        Changed
    end
    enumeration
        Open, Closed
    end
end

function log_account(a)
    disp(a.Owner);
end
