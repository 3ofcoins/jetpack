size
====

.. image:: https://travis-ci.org/mattrobenolt/size.svg?branch=master
   :target: https://travis-ci.org/mattrobenolt/size

.. image:: https://godoc.org/github.com/mattrobenolt/size?status.png
   :target: https://godoc.org/github.com/mattrobenolt/size

``size`` is a package for working with byte capacities similar to ``time.Duration``.

Usage
~~~~~

.. code-block:: go

    // Returns a Capacity which is a uint64 representing
    // 5 gigabytes in bytes.
    5*size.Gigabyte

    // Turn the string "5G" into a Capacity
    // Useful for parsing from flags.
    size.ParseCapacity("5G")

Installation
~~~~~~~~~~~~

.. code-block:: console

    $ go get github.com/mattrobenolt/size

Resources
~~~~~~~~~
* `Documentation <http://godoc.org/github.com/mattrobenolt/size>`_
