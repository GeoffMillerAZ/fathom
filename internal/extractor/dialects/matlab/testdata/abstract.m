classdef (Abstract) Shape < handle & matlab.mixin.Copyable
    properties (Abstract)
        Area
    end
    properties (SetAccess = protected)
        Name string = "shape"
        Sides (1,1) double {mustBePositive} = 3
    end
    methods (Abstract)
        r = draw(obj, ax)
        resize(obj, factor)
    end
    methods
        function obj = Shape(varargin)
            obj@handle();
            obj.Name = varargin{1};
        end
        function delete(obj)
            disp('bye');
        end
    end
    methods (Static, Access = private)
        function out = helper(x)
            out = x.^2;
            f = @(y) y + 1;
            out = f(out);
        end
    end
end
