function [result, info] = solve(problem, ...
                                options, ...
                                callback)
% SOLVE  Solve a problem.
%   [RESULT, INFO] = SOLVE(PROBLEM, OPTIONS, CALLBACK) runs the solver.
    if nargin < 3
        callback = @noop;
    end
    result = helper(problem) + scale(options);
    info.count = numel(result);
    callback(info);
end

function out = helper(p)
    out = p * 2;
    function inner(q)
        disp(q);
    end
    inner(out);
end

function scale(o)
    o = o + 1;
end

function noargs
    % No parentheses, and a help block of two comment lines
    % before the first statement.
    disp('no parens');
end

function empty()
end

function varargout = many(varargin)
    varargout{1} = varargin{1};
end
